package passwordauth

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/pocket-id/pocket-id/backend/internal/apperror"
	"github.com/pocket-id/pocket-id/backend/internal/auditlogs"
	"github.com/pocket-id/pocket-id/backend/internal/authz"
	"github.com/pocket-id/pocket-id/backend/internal/dto"
	"github.com/pocket-id/pocket-id/backend/internal/httpserver"
	"github.com/pocket-id/pocket-id/backend/internal/utils/cookie"
)

type handler struct {
	service *Service
}

// genericAccepted is returned by signup/resend/forgot so the response never reveals whether an account exists
func genericAccepted(c *gin.Context) {
	c.JSON(http.StatusAccepted, gin.H{"status": "ok"})
}

// signUp godoc
// @Summary Sign up with username, email and password (YuriID)
// @Tags Password
// @Router /api/password/signup [post]
func (h *handler) signUp(c *gin.Context) error {
	cfg, err := h.service.appConfig.GetConfig(c.Request.Context())
	if err != nil {
		return fmt.Errorf("error loading app configuration: %w", err)
	}
	var input signUpDto
	if err := httpserver.BindJSON(c, &input); err != nil {
		return err
	}
	if err := h.service.SignUp(c.Request.Context(), cfg, input, c.ClientIP(), c.Request.UserAgent()); err != nil {
		return err
	}
	genericAccepted(c)
	return nil
}

func (h *handler) resendVerification(c *gin.Context) error {
	cfg, err := h.service.appConfig.GetConfig(c.Request.Context())
	if err != nil {
		return fmt.Errorf("error loading app configuration: %w", err)
	}
	var input emailDto
	if err := httpserver.BindJSON(c, &input); err != nil {
		return err
	}
	if err := h.service.ResendVerification(c.Request.Context(), cfg, input.Email); err != nil {
		return err
	}
	genericAccepted(c)
	return nil
}

func (h *handler) verifyEmail(c *gin.Context) error {
	var input tokenDto
	if err := httpserver.BindJSON(c, &input); err != nil {
		return err
	}
	if err := h.service.VerifyEmail(c.Request.Context(), input.Token); err != nil {
		return err
	}
	c.JSON(http.StatusOK, gin.H{"status": "verified"})
	return nil
}

func (h *handler) login(c *gin.Context) error {
	cfg, err := h.service.appConfig.GetConfig(c.Request.Context())
	if err != nil {
		return fmt.Errorf("error loading app configuration: %w", err)
	}
	var input loginDto
	if err := httpserver.BindJSON(c, &input); err != nil {
		return err
	}
	browserToken, _ := c.Cookie(cookie.KnownBrowserCookieName)
	user, tokens, err := h.service.Login(c.Request.Context(), cfg, input.Identifier, input.Password, c.ClientIP(), c.Request.UserAgent(), browserToken)
	if err != nil {
		return err
	}

	var userDto dto.UserDto
	if err := dto.MapStruct(user, &userDto); err != nil {
		return err
	}
	maxAge := int(cfg.SessionDuration.AsDurationMinutes().Seconds())
	cookie.AddAccessTokenCookie(c, maxAge, tokens.AccessToken)
	if tokens.KnownBrowserToken != "" {
		cookie.AddKnownBrowserCookie(c, tokens.KnownBrowserToken, int(auditlogs.KnownBrowserLifetime.Seconds()))
	}
	c.JSON(http.StatusOK, userDto)
	return nil
}

func (h *handler) forgotPassword(c *gin.Context) error {
	cfg, err := h.service.appConfig.GetConfig(c.Request.Context())
	if err != nil {
		return fmt.Errorf("error loading app configuration: %w", err)
	}
	var input emailDto
	if err := httpserver.BindJSON(c, &input); err != nil {
		return err
	}
	if err := h.service.ForgotPassword(c.Request.Context(), cfg, input.Email); err != nil {
		return err
	}
	genericAccepted(c)
	return nil
}

func (h *handler) resetPassword(c *gin.Context) error {
	var input resetDto
	if err := httpserver.BindJSON(c, &input); err != nil {
		return err
	}
	if err := h.service.ResetPassword(c.Request.Context(), input.Token, input.Password, c.ClientIP(), c.Request.UserAgent()); err != nil {
		return err
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
	return nil
}

func (h *handler) getOwnPasswordStatus(c *gin.Context) error {
	userID := authz.PrincipalFrom(c).UserID
	if userID == "" {
		return apperror.New(apperror.Code("unauthenticated"), http.StatusUnauthorized, "Not signed in")
	}
	has, _, err := h.service.HasPassword(c.Request.Context(), userID)
	if err != nil {
		return err
	}
	c.JSON(http.StatusOK, passwordStatusDto{HasPassword: has})
	return nil
}

func (h *handler) changeOwnPassword(c *gin.Context) error {
	userID := authz.PrincipalFrom(c).UserID
	if userID == "" {
		return apperror.New(apperror.Code("unauthenticated"), http.StatusUnauthorized, "Not signed in")
	}
	var input changePasswordDto
	if err := httpserver.BindJSON(c, &input); err != nil {
		return err
	}
	if err := h.service.ChangePassword(c.Request.Context(), userID, input.CurrentPassword, input.NewPassword, c.ClientIP(), c.Request.UserAgent()); err != nil {
		return err
	}
	c.Status(http.StatusNoContent)
	return nil
}

func (h *handler) importHash(c *gin.Context) error {
	var input importHashDto
	if err := httpserver.BindJSON(c, &input); err != nil {
		return err
	}
	if err := h.service.ImportHash(c.Request.Context(), c.Param("id"), input.PasswordHash); err != nil {
		return err
	}
	c.Status(http.StatusNoContent)
	return nil
}

func (h *handler) getPasswordStatus(c *gin.Context) error {
	has, alg, err := h.service.HasPassword(c.Request.Context(), c.Param("id"))
	if err != nil {
		return err
	}
	c.JSON(http.StatusOK, passwordStatusDto{HasPassword: has, Algorithm: alg})
	return nil
}

func (h *handler) removePassword(c *gin.Context) error {
	if err := h.service.RemovePassword(c.Request.Context(), c.Param("id")); err != nil {
		return err
	}
	c.Status(http.StatusNoContent)
	return nil
}
