package v1

import (
	"errors"

	"github.com/gin-gonic/gin"

	socialapp "cixing/internal/modules/social/application"
	"cixing/internal/shared/common"
	"cixing/internal/transport/http/server/response"
)

func writeSocialError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, socialapp.ErrInvalidReactionType):
		writeFieldValidation(c, "type", "oneof", "must be one of inspired, resonated")
	case errors.Is(err, common.ErrConflict):
		writeAppError(c, response.Conflict, "social.reaction_conflict", "reaction conflict", nil)
	case errors.Is(err, common.ErrNotFound):
		writeAppError(c, response.NotFound, "social.upload_not_found", "not found", nil)
	default:
		writeInternal(c, err)
	}
}
