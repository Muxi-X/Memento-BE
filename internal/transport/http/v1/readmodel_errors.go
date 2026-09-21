package v1

import (
	"errors"

	"github.com/gin-gonic/gin"

	readmodelapp "cixing/internal/modules/readmodel/application"
	"cixing/internal/shared/common"
	"cixing/internal/transport/http/server/response"
)

func writeReadModelError(c *gin.Context, err error) {
	var validationErr *readmodelapp.UploadListValidationError
	switch {
	case errors.Is(err, readmodelapp.ErrInvalidCursor):
		writeAppError(c, response.Validation, "readmodel.invalid_cursor", "invalid cursor", nil)
	case errors.Is(err, readmodelapp.ErrCursorMismatch):
		writeAppError(c, response.Validation, "readmodel.cursor_mismatch", "cursor does not match request", nil)
	case errors.As(err, &validationErr):
		writeFieldValidation(c, validationErr.Field, validationErr.Rule, validationErr.Reason)
	case errors.Is(err, common.ErrNotFound):
		writeAppError(c, response.NotFound, "readmodel.not_found", "not found", nil)
	default:
		writeInternal(c, err)
	}
}
