//go:build unit

package passwordauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/pbkdf2"
	"gorm.io/gorm"

	"github.com/pocket-id/pocket-id/backend/internal/appconfig"
	"github.com/pocket-id/pocket-id/backend/internal/apperror"
	"github.com/pocket-id/pocket-id/backend/internal/auditlogs"
	"github.com/pocket-id/pocket-id/backend/internal/dto"
	"github.com/pocket-id/pocket-id/backend/internal/model"
	testutils "github.com/pocket-id/pocket-id/backend/internal/utils/testing"
)

type fakeSigner struct{}

func (fakeSigner) GenerateAccessToken(_ model.User, method string, _ time.Duration) (string, error) {
	return "access-token-" + method, nil
}

type fakeAudit struct{}

func (fakeAudit) CreateSignIn(context.Context, auditlogs.Event, string, string, string, string, *gorm.DB, appconfig.AppConfigValue) auditlogs.SignInResult {
	return auditlogs.SignInResult{}
}
func (fakeAudit) SendSignInNotification(context.Context, auditlogs.SignInResult) {}
func (fakeAudit) Create(context.Context, auditlogs.Event, string, string, string, auditlogs.Data, *gorm.DB) (auditlogs.AuditLog, bool) {
	return auditlogs.AuditLog{}, true
}

type fakeUsers struct{}

func (fakeUsers) CreateUserInternal(ctx context.Context, _ *appconfig.AppConfigModel, in dto.UserCreateDto, _ bool, tx *gorm.DB) (model.User, error) {
	u := model.User{Username: in.Username, Email: in.Email, EmailVerified: in.EmailVerified, FirstName: in.FirstName, DisplayName: in.DisplayName}
	err := tx.WithContext(ctx).Create(&u).Error
	return u, err
}

type sentMail struct{ kind, to, link string }

type fakeMail struct {
	mu   sync.Mutex
	sent []sentMail
}

func (f *fakeMail) add(m sentMail) { f.mu.Lock(); f.sent = append(f.sent, m); f.mu.Unlock() }
func (f *fakeMail) SendEmailVerification(_ context.Context, _ *appconfig.AppConfigModel, _, to, link string) error {
	f.add(sentMail{"verify", to, link})
	return nil
}
func (f *fakeMail) SendPasswordReset(_ context.Context, _ *appconfig.AppConfigModel, _, to, link string) error {
	f.add(sentMail{"reset", to, link})
	return nil
}
func (f *fakeMail) SendAccountNotice(_ context.Context, _ *appconfig.AppConfigModel, _, to, _, link string) error {
	f.add(sentMail{"notice", to, link})
	return nil
}
func (f *fakeMail) waitFor(t *testing.T, n int) []sentMail {
	t.Helper()
	require.Eventually(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.sent) >= n }, 5*time.Second, 10*time.Millisecond)
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentMail(nil), f.sent...)
}

func tokenFrom(t *testing.T, link string) string {
	u, err := url.Parse(link)
	require.NoError(t, err)
	return u.Query().Get("token")
}

func newTestService(t *testing.T) (*Service, *fakeMail, *gorm.DB, *appconfig.AppConfigModel) {
	db := testutils.NewDatabaseForTest(t)
	mail := &fakeMail{}
	svc := newService(Dependencies{DB: db, Signer: fakeSigner{}, AuditLog: fakeAudit{}, UserCreator: fakeUsers{}, EmailSender: mail, AppURL: "https://id.example.test"})
	cfg := &appconfig.AppConfigModel{AllowUserSignups: "open", SessionDuration: "60"}
	return svc, mail, db, cfg
}

func TestSignupVerifyLogin(t *testing.T) {
	svc, mail, db, cfg := newTestService(t)
	ctx := t.Context()

	require.Error(t, svc.SignUp(ctx, cfg, signUpDto{Username: "alice", Email: "alice@example.test", Password: "short"}, "", ""))
	require.NoError(t, svc.SignUp(ctx, cfg, signUpDto{Username: "alice", Email: "alice@example.test", Password: "a-long-password"}, "", ""))
	sent := mail.waitFor(t, 1)
	require.Equal(t, "verify", sent[0].kind)

	// Unverified: correct password is rejected with email_not_verified, wrong password stays generic
	_, _, err := svc.Login(ctx, cfg, "alice", "a-long-password", "", "", "")
	require.ErrorIs(t, err, errEmailNotVerified())
	_, _, err = svc.Login(ctx, cfg, "alice", "wrong-password!", "", "", "")
	require.ErrorIs(t, err, errInvalidCredentials())
	_, _, err = svc.Login(ctx, cfg, "nobody", "a-long-password", "", "", "")
	require.ErrorIs(t, err, errInvalidCredentials())

	tok := tokenFrom(t, sent[0].link)
	require.NoError(t, svc.VerifyEmail(ctx, tok))
	require.Error(t, svc.VerifyEmail(ctx, tok), "verification token must be single-use")

	_, tokens, err := svc.Login(ctx, cfg, "ALICE@example.test", "a-long-password", "", "", "")
	require.NoError(t, err)
	require.Equal(t, "access-token-pwd", tokens.AccessToken)

	// Duplicate signups never error and never reveal the account; the owner gets a notice instead
	require.NoError(t, svc.SignUp(ctx, cfg, signUpDto{Username: "alice2", Email: "alice@example.test", Password: "a-long-password"}, "", ""))
	require.NoError(t, svc.SignUp(ctx, cfg, signUpDto{Username: "alice", Email: "other@example.test", Password: "a-long-password"}, "", ""))
	sent = mail.waitFor(t, 3)
	require.Equal(t, "notice", sent[1].kind)
	require.Equal(t, "notice", sent[2].kind)
	var count int64
	db.Model(&model.User{}).Count(&count)
	require.EqualValues(t, 1, count)
}

func TestLegacyDjangoHashLoginAndRehash(t *testing.T) {
	svc, _, db, cfg := newTestService(t)
	ctx := t.Context()
	email := "migrated@example.test"
	u := model.User{Username: "migrated", Email: &email, EmailVerified: true}
	require.NoError(t, db.Create(&u).Error)

	key := pbkdf2.Key([]byte("original-password"), []byte("somesalt"), 1000, 32, sha256.New)
	legacy := fmt.Sprintf("pbkdf2_sha256$1000$somesalt$%s", base64.StdEncoding.EncodeToString(key))
	require.NoError(t, svc.ImportHash(ctx, u.ID, legacy))
	require.Error(t, svc.ImportHash(ctx, u.ID, "!unusable"))

	_, _, err := svc.Login(ctx, cfg, "migrated", "original-password", "", "", "")
	require.NoError(t, err)
	_, alg, err := svc.HasPassword(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, "argon2id", alg, "legacy hash must be upgraded after login")
	_, _, err = svc.Login(ctx, cfg, "migrated@example.test", "original-password", "", "", "")
	require.NoError(t, err)
}

func TestResetAndChangePassword(t *testing.T) {
	svc, mail, db, cfg := newTestService(t)
	ctx := t.Context()
	email := "bob@example.test"
	u := model.User{Username: "bob", Email: &email, EmailVerified: false}
	require.NoError(t, db.Create(&u).Error)

	require.NoError(t, svc.ForgotPassword(ctx, cfg, "nobody@example.test"))
	require.NoError(t, svc.ForgotPassword(ctx, cfg, "BOB@example.test"))
	sent := mail.waitFor(t, 1)
	require.Len(t, sent, 1)
	require.Equal(t, "reset", sent[0].kind)
	tok := tokenFrom(t, sent[0].link)

	require.Error(t, svc.ResetPassword(ctx, tok, "short", "", ""))
	require.NoError(t, svc.ResetPassword(ctx, tok, "brand-new-password", "", ""))
	err := svc.ResetPassword(ctx, tok, "another-password", "", "")
	require.ErrorContains(t, err, "invalid or expired", "reset token must be single-use")

	// Reset proves control of the inbox, so login works right away
	_, _, err = svc.Login(ctx, cfg, "bob", "brand-new-password", "", "", "")
	require.NoError(t, err)

	// Expired tokens are rejected
	require.NoError(t, svc.ForgotPassword(ctx, cfg, "bob@example.test"))
	sent = mail.waitFor(t, 2)
	svc.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	require.ErrorIs(t, svc.ResetPassword(ctx, tokenFrom(t, sent[1].link), "expired-password", "", ""), apperror.TokenInvalidOrExpired())
	svc.now = time.Now

	require.Error(t, svc.ChangePassword(ctx, u.ID, "wrong-current", "changed-password-1", "", ""))
	require.NoError(t, svc.ChangePassword(ctx, u.ID, "brand-new-password", "changed-password-1", "", ""))
	_, _, err = svc.Login(ctx, cfg, "bob", "changed-password-1", "", "", "")
	require.NoError(t, err)
}

func TestResetRevokesSessions(t *testing.T) {
	svc, mail, db, cfg := newTestService(t)
	ctx := t.Context()
	email := "carol@example.test"
	u := model.User{Username: "carol", Email: &email, EmailVerified: true}
	require.NoError(t, db.Create(&u).Error)

	before := time.Now().Add(-time.Minute)
	revoked, err := svc.IsSessionRevoked(ctx, u.ID, before)
	require.NoError(t, err)
	require.False(t, revoked, "no revocation recorded yet")

	require.NoError(t, svc.ForgotPassword(ctx, cfg, email))
	sent := mail.waitFor(t, 1)
	require.NoError(t, svc.ResetPassword(ctx, tokenFrom(t, sent[0].link), "brand-new-password", "", ""))

	revoked, err = svc.IsSessionRevoked(ctx, u.ID, before)
	require.NoError(t, err)
	require.True(t, revoked, "sessions issued before the reset must be rejected")

	revoked, err = svc.IsSessionRevoked(ctx, u.ID, time.Now().Add(2*time.Second))
	require.NoError(t, err)
	require.False(t, revoked, "sessions issued after the reset stay valid")

	// Other users are unaffected
	revoked, err = svc.IsSessionRevoked(ctx, "00000000-0000-0000-0000-000000000000", before)
	require.NoError(t, err)
	require.False(t, revoked)
}
