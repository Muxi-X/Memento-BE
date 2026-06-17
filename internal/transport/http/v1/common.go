package v1

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"cixing/internal/transport/http/server/middleware"
)

func bindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		writeValidationFields(c, bindFieldErrors(dst, err))
		return false
	}
	return true
}

func userIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(middleware.CtxUserIDKey)
	if !ok {
		return uuid.Nil, false
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func stringValue[T ~string](v *T) string {
	if v == nil {
		return ""
	}
	return string(*v)
}

func ptrIntValue(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func boolValue(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}
