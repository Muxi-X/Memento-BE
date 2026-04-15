package v1

import (
	"errors"

	"github.com/gin-gonic/gin"

	"cixing/internal/shared/common"
	"cixing/internal/transport/http/server/response"
)

func writeReadModelError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, common.ErrNotFound):
		writeAppError(c, response.NotFound, "readmodel.not_found", "not found", nil)
	default:
		writeInternal(c, err)
	}
}
