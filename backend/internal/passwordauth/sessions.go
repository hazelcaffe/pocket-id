package passwordauth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/pocket-id/pocket-id/backend/internal/oidc"
)

// revokeSessions signs a user out everywhere: every Pocket ID session token issued up to now stops being
// accepted, and the refresh (and paired access) tokens of every app the user authorized are revoked so connected apps cannot mint new tokens for the user.
func (s *Service) revokeSessions(ctx context.Context, tx *gorm.DB, userID string) error {
	now := s.now().Unix()
	err := tx.WithContext(ctx).Exec(
		"INSERT INTO yuriid_session_revocations (user_id, revoked_at) VALUES (?, ?) "+
			"ON CONFLICT (user_id) DO UPDATE SET revoked_at = excluded.revoked_at",
		userID, now).Error
	if err != nil {
		return err
	}
	var clientIDs []string
	err = tx.WithContext(ctx).
		Raw("SELECT client_id FROM user_authorized_oidc_clients WHERE user_id = ?", userID).
		Scan(&clientIDs).Error
	if err != nil {
		return err
	}
	for _, clientID := range clientIDs {
		if err := oidc.RevokeUserClientSessions(ctx, tx, userID, clientID); err != nil {
			return err
		}
	}
	return nil
}

// IsSessionRevoked reports whether a session token for userID issued at issuedAt was revoked.
// Token "iat" has one-second resolution, so a token from the same second as the revocation is rejected too.
func (s *Service) IsSessionRevoked(ctx context.Context, userID string, issuedAt time.Time) (bool, error) {
	var revokedAt int64
	err := s.db.WithContext(ctx).
		Raw("SELECT revoked_at FROM yuriid_session_revocations WHERE user_id = ?", userID).
		Row().Scan(&revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return issuedAt.IsZero() || issuedAt.Unix() <= revokedAt, nil
}

// IsSessionRevoked exposes the revocation check to the session authenticator
func (m *Module) IsSessionRevoked(ctx context.Context, userID string, issuedAt time.Time) (bool, error) {
	return m.service.IsSessionRevoked(ctx, userID, issuedAt)
}
