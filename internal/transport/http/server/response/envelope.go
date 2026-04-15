package response

import (
	"github.com/gin-gonic/gin"

	"cixing/internal/transport/http/server/middleware"
)

// 统一输出 JSON 响应时，将Request ID 放入响应头中
func JSON(c *gin.Context, status int, body any) {
	rid := middleware.GetRequestID(c)
	if rid != "" {
		c.Header(middleware.HeaderRequestID, rid)
	}
	c.JSON(status, body)
}
