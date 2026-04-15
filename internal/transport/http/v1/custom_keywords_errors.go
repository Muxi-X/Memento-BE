package v1

import (
	"errors"

	"github.com/gin-gonic/gin"

	customapp "cixing/internal/modules/customkeywords/application"
	"cixing/internal/shared/common"
	"cixing/internal/transport/http/server/response"
)

func writeCustomKeywordError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, customapp.ErrInvalidKeywordText):
		writeFieldValidation(c, "text", "required", "must not be empty")
	case errors.Is(err, customapp.ErrInvalidTargetImageCount):
		writeFieldValidation(c, "target_image_count", "min", "must be greater than or equal to 1")
	case errors.Is(err, customapp.ErrInvalidInput):
		writeAppError(c, response.Validation, "custom_keyword.invalid_input", "invalid custom keyword request", nil)
	case errors.Is(err, common.ErrNotFound):
		writeAppError(c, response.NotFound, "custom_keyword.not_found", "not found", nil)
	case errors.Is(err, common.ErrConflict):
		writeAppError(c, response.Conflict, "custom_keyword.conflict", "conflict", nil)
	default:
		writeInternal(c, err)
	}
}
