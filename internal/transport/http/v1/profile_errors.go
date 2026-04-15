package v1

import (
	"errors"

	"github.com/gin-gonic/gin"

	profileapp "cixing/internal/modules/profile/application"
	"cixing/internal/shared/common"
	"cixing/internal/transport/http/server/response"
)

func writeProfileError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, profileapp.ErrInvalidNickname):
		writeFieldValidation(c, "nickname", "length", "must be between 1 and 40 characters")
	case errors.Is(err, profileapp.ErrInvalidInput):
		writeAppError(c, response.Validation, "profile.invalid_input", "invalid profile request", nil)
	case errors.Is(err, common.ErrNotFound):
		writeAppError(c, response.NotFound, "profile.not_found", "not found", nil)
	default:
		writeInternal(c, err)
	}
}
