package v1

import (
	"errors"

	"github.com/gin-gonic/gin"

	officialapp "cixing/internal/modules/official/application"
	"cixing/internal/shared/common"
	"cixing/internal/transport/http/server/response"
)

func writeOfficialPromptError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, officialapp.ErrInvalidPromptKind):
		writeFieldValidation(c, "kind", "oneof", "must be one of intuition, structure, concept")
	case errors.Is(err, officialapp.ErrUnauthenticatedUser):
		writeUnauthorized(c)
	case errors.Is(err, officialapp.ErrPromptDateChanged):
		writeAppError(c, response.Conflict, "official.prompt_date_changed", "prompt date changed", nil)
	case errors.Is(err, officialapp.ErrKeywordMismatch):
		writeAppError(c, response.Conflict, "official.keyword_mismatch", "keyword mismatch", nil)
	case errors.Is(err, officialapp.ErrPromptNotAvailable):
		writeAppError(c, response.NotFound, "official.prompt_not_available", "prompt not available", nil)
	case errors.Is(err, common.ErrNotFound):
		writeAppError(c, response.NotFound, "official.prompt_not_available", "prompt not available", nil)
	default:
		writeInternal(c, err)
	}
}
