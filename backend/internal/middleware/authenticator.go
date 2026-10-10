package middleware

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/pocket-id/pocket-id/backend/internal/apikey"
	"github.com/pocket-id/pocket-id/backend/internal/apperror"
	"github.com/pocket-id/pocket-id/backend/internal/authz"
	"github.com/pocket-id/pocket-id/backend/internal/service"
	"github.com/pocket-id/pocket-id/backend/internal/utils/cookie"
)

// #nosec G101 -- this is the name of the header that carries the API key, not a credential
const apiKeyHeader = "X-API-Key"

// NewAuthorization creates the authorization middleware with every credential Pocket ID accepts
// A browser session is tried before an API key, so a signed-in browser is never mistaken for an API client
func NewAuthorization(apiKeyModule *apikey.Module, userService *service.UserService, jwtService *service.JwtService, revocation ...SessionRevocationChecker) *authz.Middleware {
	session := NewSessionAuthenticator(jwtService, userService)
	if len(revocation) > 0 {
		session.revocation = revocation[0]
	}
	return authz.NewMiddleware(
		session,
		NewAPIKeyAuthenticator(apiKeyModule),
	)
}

// SessionAuthenticator authenticates the session access token Pocket ID issues after sign-in
type SessionAuthenticator struct {
	jwtService  *service.JwtService
	userService *service.UserService
	revocation  SessionRevocationChecker
}

// SessionRevocationChecker reports whether a user's session tokens issued at a given time were revoked
// (YuriID: a password reset signs the user out everywhere)
type SessionRevocationChecker interface {
	IsSessionRevoked(ctx context.Context, userID string, issuedAt time.Time) (bool, error)
}

func NewSessionAuthenticator(jwtService *service.JwtService, userService *service.UserService) *SessionAuthenticator {
	return &SessionAuthenticator{jwtService: jwtService, userService: userService}
}

func (a *SessionAuthenticator) Kind() authz.PrincipalKind {
	return authz.KindSession
}

func (a *SessionAuthenticator) Present(c *gin.Context) bool {
	return sessionToken(c) != ""
}

func (a *SessionAuthenticator) Authenticate(c *gin.Context) (*authz.Principal, error) {
	// Verify the token signature, audience and type
	token, err := a.jwtService.VerifyAccessToken(sessionToken(c))
	if err != nil {
		return nil, apperror.NotSignedIn()
	}
	authenticationMethod, err := a.jwtService.GetAuthenticationMethod(token)
	if err != nil {
		return nil, apperror.NotSignedIn()
	}
	authenticationTime, _ := token.IssuedAt()

	subject, ok := token.Subject()
	if !ok {
		return nil, apperror.TokenInvalid()
	}

	// Load the user so disabling an account or changing its admin flag takes effect before the token expires
	user, err := a.userService.GetUser(c, subject)
	if err != nil {
		return nil, apperror.NotSignedIn()
	}
	if user.Disabled {
		return nil, apperror.UserDisabled()
	}
	if a.revocation != nil {
		revoked, err := a.revocation.IsSessionRevoked(c.Request.Context(), user.ID, authenticationTime)
		if err != nil {
			// Fail closed: an unreadable revocation list must not keep a possibly revoked session alive
			slog.ErrorContext(c.Request.Context(), "Failed to check session revocation", slog.Any("error", err))
			return nil, apperror.NotSignedIn()
		}
		if revoked {
			return nil, apperror.NotSignedIn()
		}
	}

	return &authz.Principal{
		Kind:                 authz.KindSession,
		UserID:               user.ID,
		Scopes:               authz.UserScopes(user.IsAdmin, authz.KindSession),
		AuthenticationMethod: authenticationMethod,
		AuthenticationTime:   authenticationTime,
	}, nil
}

// sessionToken reads the session access token from its cookie, or from the Authorization header when the cookie is absent
// An invalid cookie deliberately does not fall back to the header
func sessionToken(c *gin.Context) string {
	if accessToken, err := c.Cookie(cookie.AccessTokenCookieName); err == nil {
		return accessToken
	}

	_, accessToken, _ := strings.Cut(c.GetHeader("Authorization"), " ")
	return accessToken
}

// APIKeyAuthenticator authenticates a personal API key sent in the X-API-Key header
type APIKeyAuthenticator struct {
	apiKeyModule *apikey.Module
}

func NewAPIKeyAuthenticator(apiKeyModule *apikey.Module) *APIKeyAuthenticator {
	return &APIKeyAuthenticator{apiKeyModule: apiKeyModule}
}

func (a *APIKeyAuthenticator) Kind() authz.PrincipalKind {
	return authz.KindAPIKey
}

func (a *APIKeyAuthenticator) Present(c *gin.Context) bool {
	return c.GetHeader(apiKeyHeader) != ""
}

func (a *APIKeyAuthenticator) Authenticate(c *gin.Context) (*authz.Principal, error) {
	user, err := a.apiKeyModule.ValidateApiKey(c.Request.Context(), c.GetHeader(apiKeyHeader))
	if err != nil {
		return nil, apperror.NotSignedIn()
	}
	if user.Disabled {
		return nil, apperror.UserDisabled()
	}

	return &authz.Principal{
		Kind:   authz.KindAPIKey,
		UserID: user.ID,
		Scopes: authz.UserScopes(user.IsAdmin, authz.KindAPIKey),
	}, nil
}
