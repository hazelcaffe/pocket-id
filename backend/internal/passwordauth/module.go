// Package passwordauth adds YuriID's username/email + password authentication on top of Pocket ID:
// open self-registration with email verification, password sign-in (argon2id, with transparent upgrade of
// imported Django/Authentik hashes), forgot/reset password via single-use emailed links, and change password.
package passwordauth

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/pocket-id/pocket-id/backend/internal/appconfig"
	"github.com/pocket-id/pocket-id/backend/internal/auditlogs"
	"github.com/pocket-id/pocket-id/backend/internal/authz"
	"github.com/pocket-id/pocket-id/backend/internal/dto"
	"github.com/pocket-id/pocket-id/backend/internal/httpserver"
	"github.com/pocket-id/pocket-id/backend/internal/model"
)

type TokenService interface {
	GenerateAccessToken(user model.User, authenticationMethod string, sessionDuration time.Duration) (string, error)
}

type AuditLogger interface {
	CreateSignIn(ctx context.Context, event auditlogs.Event, ipAddress, userAgent, userID, browserToken string, tx *gorm.DB, notificationMode appconfig.AppConfigValue) auditlogs.SignInResult
	SendSignInNotification(ctx context.Context, result auditlogs.SignInResult)
	Create(ctx context.Context, event auditlogs.Event, ipAddress, userAgent, userID string, data auditlogs.Data, tx *gorm.DB) (auditlogs.AuditLog, bool)
}

type UserCreator interface {
	CreateUserInternal(ctx context.Context, cfg *appconfig.AppConfigModel, input dto.UserCreateDto, isLdapSync bool, tx *gorm.DB) (model.User, error)
}

type EmailSender interface {
	SendEmailVerification(ctx context.Context, dbConfig *appconfig.AppConfigModel, userFullName, userEmail, verificationLink string) error
	SendPasswordReset(ctx context.Context, dbConfig *appconfig.AppConfigModel, userFullName, userEmail, resetLink string) error
	SendAccountNotice(ctx context.Context, dbConfig *appconfig.AppConfigModel, userFullName, userEmail, message, link string) error
}

type Dependencies struct {
	DB          *gorm.DB
	Signer      TokenService
	AuditLog    AuditLogger
	UserCreator UserCreator
	EmailSender EmailSender
	AppConfig   appconfig.AppConfigResolver
	AppURL      string
}

type Module struct {
	service *Service
	handler *handler
}

func New(deps Dependencies) *Module {
	svc := newService(deps)
	return &Module{service: svc, handler: &handler{service: svc}}
}

// RateLimits groups the per-IP limiters used by the public endpoints
type RateLimits struct {
	Signup gin.HandlerFunc
	Login  gin.HandlerFunc
	Email  gin.HandlerFunc // forgot password, resend verification
	Token  gin.HandlerFunc // verify email, reset password
	Change gin.HandlerFunc // change own password
}

func (m *Module) RegisterRoutes(r *authz.Router, rl RateLimits) {
	pub := r.Public()
	pub.POST("/password/signup", rl.Signup, httpserver.Handle(m.handler.signUp))
	pub.POST("/password/resend-verification", rl.Email, httpserver.Handle(m.handler.resendVerification))
	pub.POST("/password/verify-email", rl.Token, httpserver.Handle(m.handler.verifyEmail))
	pub.POST("/password/login", rl.Login, httpserver.Handle(m.handler.login))
	pub.POST("/password/forgot", rl.Email, httpserver.Handle(m.handler.forgotPassword))
	pub.POST("/password/reset", rl.Token, httpserver.Handle(m.handler.resetPassword))

	r.GET("/users/me/password", authz.AccountRead, httpserver.Handle(m.handler.getOwnPasswordStatus))
	r.PUT("/users/me/password", authz.AccountWrite, rl.Change, httpserver.Handle(m.handler.changeOwnPassword))

	r.PUT("/users/:id/password-hash", authz.UsersWrite, httpserver.Handle(m.handler.importHash))
	r.GET("/users/:id/password-status", authz.UsersRead, httpserver.Handle(m.handler.getPasswordStatus))
	r.DELETE("/users/:id/password", authz.UsersWrite, httpserver.Handle(m.handler.removePassword))
}
