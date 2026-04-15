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
	case errors.Is(err, common.ErrNotFound):
		writeAppError(c, response.NotFound, "official.keyword_not_found", "not found", nil)
	default:
		writeInternal(c, err)
	}
}
