package passwordauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"gorm.io/gorm"

	"github.com/pocket-id/pocket-id/backend/internal/appconfig"
	"github.com/pocket-id/pocket-id/backend/internal/apperror"
	"github.com/pocket-id/pocket-id/backend/internal/auditlogs"
	"github.com/pocket-id/pocket-id/backend/internal/dto"
	"github.com/pocket-id/pocket-id/backend/internal/model"
)

const (
	// authenticationMethodPassword is emitted in the access token's "amr" claim (RFC 8176)
	authenticationMethodPassword = "pwd"

	purposeVerifyEmail   = "verify_email"
	purposeResetPassword = "reset_password"

	verifyEmailTTL   = 24 * time.Hour
	resetPasswordTTL = time.Hour
)

const (
	CodeInvalidCredentials apperror.Code = "invalid_credentials"
	CodeEmailNotVerified   apperror.Code = "email_not_verified"
	CodeWeakPassword       apperror.Code = "weak_password"
	CodeWrongPassword      apperror.Code = "wrong_current_password"
)

func errInvalidCredentials() *apperror.Error {
	return apperror.New(CodeInvalidCredentials, http.StatusUnauthorized, "Invalid username/email or password")
}

func errEmailNotVerified() *apperror.Error {
	return apperror.New(CodeEmailNotVerified, http.StatusForbidden, "Please verify your email address before signing in")
}

type Service struct {
	db        *gorm.DB
	signer    TokenService
	auditLog  AuditLogger
	users     UserCreator
	email     EmailSender
	appConfig appconfig.AppConfigResolver
	appURL    string
	now       func() time.Time
}

func newService(deps Dependencies) *Service {
	return &Service{
		db:        deps.DB,
		signer:    deps.Signer,
		auditLog:  deps.AuditLog,
		users:     deps.UserCreator,
		email:     deps.EmailSender,
		appConfig: deps.AppConfig,
		appURL:    strings.TrimRight(deps.AppURL, "/"),
		now:       time.Now,
	}
}

// ValidateNewPassword enforces the password policy
func ValidateNewPassword(password string) error {
	n := utf8.RuneCountInString(password)
	if n < MinPasswordLength {
		return apperror.New(CodeWeakPassword, http.StatusBadRequest, "Password must be at least 10 characters long")
	}
	if len(password) > MaxPasswordLength {
		return apperror.New(CodeWeakPassword, http.StatusBadRequest, "Password is too long")
	}
	if strings.TrimSpace(password) == "" {
		return apperror.New(CodeWeakPassword, http.StatusBadRequest, "Password must not be blank")
	}
	return nil
}

// ---- password storage ----

type passwordRow struct {
	UserID       string
	PasswordHash string
}

func (s *Service) getHash(ctx context.Context, tx *gorm.DB, userID string) (string, bool, error) {
	var row passwordRow
	res := tx.WithContext(ctx).Raw("SELECT user_id, password_hash FROM yuriid_user_passwords WHERE user_id = ?", userID).Scan(&row)
	if res.Error != nil {
		return "", false, res.Error
	}
	if res.RowsAffected == 0 || row.PasswordHash == "" {
		return "", false, nil
	}
	return row.PasswordHash, true, nil
}

func (s *Service) setHash(ctx context.Context, tx *gorm.DB, userID, hash string) error {
	return tx.WithContext(ctx).Exec(
		`INSERT INTO yuriid_user_passwords (user_id, password_hash, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT (user_id) DO UPDATE SET password_hash = excluded.password_hash, updated_at = excluded.updated_at`,
		userID, hash, s.now().Unix(),
	).Error
}

func (s *Service) setPassword(ctx context.Context, tx *gorm.DB, userID, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.setHash(ctx, tx, userID, hash)
}

// ---- single-use email tokens ----

func newToken() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Service) issueToken(ctx context.Context, userID, purpose string, ttl time.Duration) (string, error) {
	token, tokenHash, err := newToken()
	if err != nil {
		return "", err
	}
	id := uuid.NewV4()
	now := s.now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Only the most recent token for a purpose stays valid
		if err := tx.Exec("UPDATE yuriid_auth_tokens SET used_at = ? WHERE user_id = ? AND purpose = ? AND used_at IS NULL", now.Unix(), userID, purpose).Error; err != nil {
			return err
		}
		// Housekeeping: drop expired tokens
		if err := tx.Exec("DELETE FROM yuriid_auth_tokens WHERE expires_at < ?", now.Add(-7*24*time.Hour).Unix()).Error; err != nil {
			return err
		}
		return tx.Exec("INSERT INTO yuriid_auth_tokens (id, user_id, purpose, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			id.String(), userID, purpose, tokenHash, now.Add(ttl).Unix(), now.Unix()).Error
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// consumeToken atomically marks a token used and returns its user; it fails if the token is unknown, expired or already used
func (s *Service) consumeToken(ctx context.Context, tx *gorm.DB, token, purpose string) (string, error) {
	if len(token) < 20 || len(token) > 100 {
		return "", apperror.TokenInvalidOrExpired()
	}
	tokenHash := hashToken(token)
	now := s.now().Unix()

	var userID string
	res := tx.WithContext(ctx).Raw("SELECT user_id FROM yuriid_auth_tokens WHERE token_hash = ? AND purpose = ? AND used_at IS NULL AND expires_at > ?", tokenHash, purpose, now).Scan(&userID)
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected == 0 || userID == "" {
		return "", apperror.TokenInvalidOrExpired()
	}
	upd := tx.WithContext(ctx).Exec("UPDATE yuriid_auth_tokens SET used_at = ? WHERE token_hash = ? AND purpose = ? AND used_at IS NULL AND expires_at > ?", now, tokenHash, purpose, now)
	if upd.Error != nil {
		return "", upd.Error
	}
	if upd.RowsAffected != 1 {
		return "", apperror.TokenInvalidOrExpired()
	}
	return userID, nil
}

// ---- lookups ----

func (s *Service) findByIdentifier(ctx context.Context, identifier string) (model.User, bool, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return model.User{}, false, nil
	}
	// An exact username match wins; otherwise fall back to the email address
	user, ok, err := s.findByUsername(ctx, identifier)
	if err != nil || ok {
		return user, ok, err
	}
	if strings.Contains(identifier, "@") {
		return s.findByEmail(ctx, identifier)
	}
	return model.User{}, false, nil
}

func (s *Service) findByEmail(ctx context.Context, email string) (model.User, bool, error) {
	var user model.User
	err := s.db.WithContext(ctx).Where("LOWER(email) = LOWER(?)", strings.TrimSpace(email)).Limit(1).Find(&user).Error
	if err != nil {
		return model.User{}, false, err
	}
	return user, user.ID != "", nil
}

func (s *Service) findByUsername(ctx context.Context, username string) (model.User, bool, error) {
	var user model.User
	err := s.db.WithContext(ctx).Where("LOWER(username) = LOWER(?)", strings.TrimSpace(username)).Limit(1).Find(&user).Error
	if err != nil {
		return model.User{}, false, err
	}
	return user, user.ID != "", nil
}

// ---- background mail (keeps response timing independent of whether an account exists) ----

func (s *Service) sendAsync(ctx context.Context, what string, fn func(ctx context.Context) error) {
	bg := context.WithoutCancel(ctx)
	go func() {
		ctx, cancel := context.WithTimeout(bg, 30*time.Second)
		defer cancel()
		if err := fn(ctx); err != nil {
			slog.ErrorContext(ctx, "YuriID: failed to send email", slog.String("kind", what), slog.Any("error", err))
		}
	}()
}

func emailOf(u model.User) string {
	if u.Email == nil {
		return ""
	}
	return *u.Email
}

func nameOf(u model.User) string {
	if n := strings.TrimSpace(u.FullName()); n != "" {
		return n
	}
	return u.Username
}

// ---- flows ----

// SignUp creates an unverified account with a password and emails a verification link.
// The caller always gets the same generic response, whether or not the username or email were already taken.
func (s *Service) SignUp(ctx context.Context, cfg *appconfig.AppConfigModel, input signUpDto, ipAddress, userAgent string) error {
	if cfg.AllowUserSignups.String() != "open" {
		return apperror.OpenSignupDisabled()
	}
	if err := ValidateNewPassword(input.Password); err != nil {
		return err
	}

	// Hash before any lookup so timing is the same for new and existing accounts
	hash, err := HashPassword(input.Password)
	if err != nil {
		return err
	}

	existingByEmail, emailTaken, err := s.findByEmail(ctx, input.Email)
	if err != nil {
		return err
	}
	if emailTaken {
		// Tell the owner of the address instead of the requester
		u := existingByEmail
		if !u.EmailVerified {
			// An earlier signup that was never verified: send a fresh verification link
			s.sendVerification(ctx, cfg, u)
			return nil
		}
		s.sendAsync(ctx, "signup-existing", func(ctx context.Context) error {
			return s.email.SendAccountNotice(ctx, cfg, nameOf(u), emailOf(u),
				"Someone tried to create a new account with this email address, but you already have one. If that was you, just sign in, or use \"Forgot password?\" on the sign-in page. If it wasn't you, you can ignore this email.",
				s.appURL+"/login")
		})
		return nil
	}

	_, usernameTaken, err := s.findByUsername(ctx, input.Username)
	if err != nil {
		return err
	}
	if usernameTaken {
		email := strings.TrimSpace(input.Email)
		username := input.Username
		s.sendAsync(ctx, "signup-username-taken", func(ctx context.Context) error {
			return s.email.SendAccountNotice(ctx, cfg, username, email,
				"We couldn't create your account because the username \""+username+"\" is already taken. Please sign up again with a different username.",
				s.appURL+"/signup")
		})
		return nil
	}

	email := strings.TrimSpace(input.Email)
	var created model.User
	tx := s.db.Begin()
	defer tx.Rollback()
	created, err = s.users.CreateUserInternal(ctx, cfg, dto.UserCreateDto{
		Username:      input.Username,
		Email:         &email,
		EmailVerified: false,
		FirstName:     input.Username,
		DisplayName:   input.Username,
	}, false, tx)
	if err != nil {
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.HTTPStatus() == http.StatusConflict {
			// Lost a race with another signup; stay generic
			return nil
		}
		return err
	}
	if err := s.setHash(ctx, tx, created.ID, hash); err != nil {
		return err
	}
	s.auditLog.Create(ctx, auditlogs.EventAccountCreated, ipAddress, userAgent, created.ID, auditlogs.Data{"method": "password_signup"}, tx)
	if err := tx.Commit().Error; err != nil {
		return err
	}

	s.sendVerification(ctx, cfg, created)
	return nil
}

func (s *Service) sendVerification(ctx context.Context, cfg *appconfig.AppConfigModel, u model.User) {
	if emailOf(u) == "" {
		return
	}
	token, err := s.issueToken(ctx, u.ID, purposeVerifyEmail, verifyEmailTTL)
	if err != nil {
		slog.ErrorContext(ctx, "YuriID: failed to issue verification token", slog.Any("error", err))
		return
	}
	link := s.appURL + "/verify-account?token=" + token
	s.sendAsync(ctx, "verify-email", func(ctx context.Context) error {
		return s.email.SendEmailVerification(ctx, cfg, nameOf(u), emailOf(u), link)
	})
}

// ResendVerification sends a new verification link if the address belongs to an unverified account (always silent)
func (s *Service) ResendVerification(ctx context.Context, cfg *appconfig.AppConfigModel, email string) error {
	u, ok, err := s.findByEmail(ctx, email)
	if err != nil || !ok || u.EmailVerified || u.Disabled {
		return err
	}
	s.sendVerification(ctx, cfg, u)
	return nil
}

// VerifyEmail consumes a verification token and marks the email verified
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userID, err := s.consumeToken(ctx, tx, token, purposeVerifyEmail)
		if err != nil {
			return err
		}
		return tx.Model(&model.User{}).Where("id = ?", userID).Update("email_verified", true).Error
	})
}

// Login verifies username/email + password and returns the user.
// Legacy (imported) hashes are transparently upgraded to argon2id after a successful login.
func (s *Service) Login(ctx context.Context, cfg *appconfig.AppConfigModel, identifier, password, ipAddress, userAgent, browserToken string) (model.User, model.LoginTokens, error) {
	if password == "" || len(password) > MaxPasswordLength {
		burnTime("x")
		return model.User{}, model.LoginTokens{}, errInvalidCredentials()
	}

	user, found, err := s.findByIdentifier(ctx, identifier)
	if err != nil {
		return model.User{}, model.LoginTokens{}, err
	}
	var hash string
	var hasHash bool
	if found {
		hash, hasHash, err = s.getHash(ctx, s.db, user.ID)
		if err != nil {
			return model.User{}, model.LoginTokens{}, err
		}
	}
	if !found || !hasHash {
		burnTime(password)
		return model.User{}, model.LoginTokens{}, errInvalidCredentials()
	}

	ok, needsRehash, err := VerifyPassword(password, hash)
	if err != nil {
		slog.WarnContext(ctx, "YuriID: stored password hash could not be verified", slog.String("algorithm", HashAlgorithm(hash)))
		return model.User{}, model.LoginTokens{}, errInvalidCredentials()
	}
	if !ok {
		return model.User{}, model.LoginTokens{}, errInvalidCredentials()
	}

	// Only reveal account state after the correct password was given
	if user.Disabled {
		return model.User{}, model.LoginTokens{}, apperror.UserDisabled()
	}
	if emailOf(user) != "" && !user.EmailVerified {
		return model.User{}, model.LoginTokens{}, errEmailNotVerified()
	}

	if needsRehash {
		newHash, err := HashPassword(password)
		if err == nil {
			// Conditional on the old hash so a concurrent password change is never overwritten
			err = s.db.WithContext(ctx).Exec("UPDATE yuriid_user_passwords SET password_hash = ?, updated_at = ? WHERE user_id = ? AND password_hash = ?",
				newHash, s.now().Unix(), user.ID, hash).Error
		}
		if err != nil {
			slog.ErrorContext(ctx, "YuriID: failed to upgrade legacy password hash", slog.Any("error", err))
		} else {
			slog.InfoContext(ctx, "YuriID: upgraded password hash to argon2id", slog.String("from", HashAlgorithm(hash)))
		}
	}

	accessToken, err := s.signer.GenerateAccessToken(user, authenticationMethodPassword, cfg.SessionDuration.AsDurationMinutes())
	if err != nil {
		return model.User{}, model.LoginTokens{}, err
	}
	signIn := s.auditLog.CreateSignIn(ctx, auditlogs.EventPasswordSignIn, ipAddress, userAgent, user.ID, browserToken, s.db, cfg.EmailLoginNotificationMode)
	s.auditLog.SendSignInNotification(ctx, signIn)

	return user, model.LoginTokens{AccessToken: accessToken, KnownBrowserToken: signIn.KnownBrowserToken}, nil
}

// ForgotPassword emails a reset link if the address belongs to an enabled account (always silent)
func (s *Service) ForgotPassword(ctx context.Context, cfg *appconfig.AppConfigModel, email string) error {
	u, ok, err := s.findByEmail(ctx, email)
	if err != nil || !ok || u.Disabled {
		return err
	}
	token, err := s.issueToken(ctx, u.ID, purposeResetPassword, resetPasswordTTL)
	if err != nil {
		return err
	}
	link := s.appURL + "/reset-password?token=" + token
	s.sendAsync(ctx, "password-reset", func(ctx context.Context) error {
		return s.email.SendPasswordReset(ctx, cfg, nameOf(u), emailOf(u), link)
	})
	return nil
}

// ResetPassword consumes a reset token and sets a new password.
// Receiving the link proves control of the email address, so the email is also marked verified.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword, ipAddress, userAgent string) error {
	if err := ValidateNewPassword(newPassword); err != nil {
		return err
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userID, err := s.consumeToken(ctx, tx, token, purposeResetPassword)
		if err != nil {
			return err
		}
		if err := s.setHash(ctx, tx, userID, hash); err != nil {
			return err
		}
		if err := tx.Exec("UPDATE yuriid_auth_tokens SET used_at = ? WHERE user_id = ? AND purpose = ? AND used_at IS NULL", s.now().Unix(), userID, purposeResetPassword).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Update("email_verified", true).Error; err != nil {
			return err
		}
		s.auditLog.Create(ctx, auditlogs.Event("PASSWORD_RESET"), ipAddress, userAgent, userID, auditlogs.Data{}, tx)
		return nil
	})
}

// ChangePassword changes the signed-in user's password; the current one is required when one is set
func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword, ipAddress, userAgent string) error {
	if err := ValidateNewPassword(newPassword); err != nil {
		return err
	}
	hash, has, err := s.getHash(ctx, s.db, userID)
	if err != nil {
		return err
	}
	if has {
		ok, _, err := VerifyPassword(currentPassword, hash)
		if err != nil || !ok {
			return apperror.New(CodeWrongPassword, http.StatusBadRequest, "Current password is incorrect")
		}
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.setPassword(ctx, tx, userID, newPassword); err != nil {
			return err
		}
		s.auditLog.Create(ctx, auditlogs.Event("PASSWORD_CHANGED"), ipAddress, userAgent, userID, auditlogs.Data{}, tx)
		return nil
	})
}

// HasPassword reports whether a user has a password set
func (s *Service) HasPassword(ctx context.Context, userID string) (bool, string, error) {
	hash, has, err := s.getHash(ctx, s.db, userID)
	if err != nil || !has {
		return false, "", err
	}
	return true, HashAlgorithm(hash), nil
}

// ImportHash stores an already-hashed (possibly legacy) password for a user; admin-only, used by the Authentik migration
func (s *Service) ImportHash(ctx context.Context, userID, encoded string) error {
	if err := ValidateImportableHash(encoded); err != nil {
		return apperror.ValidationMessage("Unsupported or unusable password hash format")
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return apperror.NotFound("user")
	}
	return s.setHash(ctx, s.db, userID, encoded)
}

// RemovePassword removes a user's password (admin)
func (s *Service) RemovePassword(ctx context.Context, userID string) error {
	return s.db.WithContext(ctx).Exec("DELETE FROM yuriid_user_passwords WHERE user_id = ?", userID).Error
}
