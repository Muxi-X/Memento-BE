package v1

import (
	"errors"

	"github.com/gin-gonic/gin"

	publishingapp "cixing/internal/modules/publishing/application"
	dpub "cixing/internal/modules/publishing/domain"
	"cixing/internal/shared/common"
	"cixing/internal/transport/http/server/response"
)

func writeUploadPublishError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, publishingapp.ErrInvalidUploadPublishInput):
		writeAppError(c, response.Validation, "publish.invalid_input", "invalid upload publish request", nil)
	case errors.Is(err, publishingapp.ErrUploadPublishExpired),
		errors.Is(err, dpub.ErrExpired):
		writeAppError(c, response.Conflict, "publish.session_expired", "upload publish session expired", nil)
	case errors.Is(err, dpub.ErrInvalidState):
		writeAppError(c, response.Conflict, "publish.invalid_state", "invalid upload publish session state", nil)
	case errors.Is(err, dpub.ErrInvalidContext):
		writeAppError(c, response.Conflict, "publish.invalid_context", "invalid upload publish context", nil)
	case errors.Is(err, dpub.ErrKeywordMismatch):
		writeAppError(c, response.Conflict, "publish.keyword_mismatch", "upload publish keyword mismatch", nil)
	case errors.Is(err, dpub.ErrInvalidAsset):
		writeAppError(c, response.Conflict, "publish.invalid_asset", "invalid upload publish asset", nil)
	case errors.Is(err, common.ErrConflict):
		writeAppError(c, response.Conflict, "publish.conflict", "upload publish conflict", nil)
	case errors.Is(err, common.ErrNotFound):
		writeAppError(c, response.NotFound, "publish.not_found", "not found", nil)
	default:
		writeInternal(c, err)
	}
}
