package v1

import (
	"errors"

	"github.com/gin-gonic/gin"

	authapp "cixing/internal/modules/auth/application"
	"cixing/internal/shared/common"
	"cixing/internal/transport/http/server/response"
)

func writeAuthError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, authapp.ErrInvalidEmail):
		writeAppError(c, response.Validation, "auth.invalid_email", "invalid email", nil)
	case errors.Is(err, authapp.ErrInvalidVerificationCode):
		writeFieldValidation(c, "code", "pattern", "must be exactly 6 digits")
	case errors.Is(err, authapp.ErrPasswordTooShort):
		writeAppError(c, response.Validation, "auth.password_too_short", "password too short", nil)
	case errors.Is(err, authapp.ErrCodeVerificationFailed):
		writeAppError(c, response.Validation, "auth.code_verification_failed", "code verification failed", nil)
	case errors.Is(err, authapp.ErrInvalidSignupToken):
		writeAppError(c, response.Validation, "auth.signup_token_invalid", "invalid signup token", nil)
	case errors.Is(err, authapp.ErrInvalidResetToken):
		writeAppError(c, response.Validation, "auth.reset_token_invalid", "invalid reset token", nil)
	case errors.Is(err, authapp.ErrInvalidToken):
		writeAppError(c, response.Validation, "auth.invalid_token", "invalid token", nil)
	case errors.Is(err, authapp.ErrInvalidCredentials):
		writeAppError(c, response.Unauthorized, "auth.invalid_credentials", "invalid credentials", nil)
	case errors.Is(err, common.ErrConflict):
		writeAppError(c, response.Conflict, "auth.email_conflict", "email already exists", nil)
	case errors.Is(err, common.ErrNotFound):
		writeAppError(c, response.NotFound, "auth.not_found", "not found", nil)
	default:
		writeInternal(c, err)
	}
}
