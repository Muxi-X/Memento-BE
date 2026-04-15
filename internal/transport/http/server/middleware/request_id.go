package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 挂 Request ID 的中间件

const (
	HeaderRequestID = "X-Request-ID" // 请求头

	ctxRequestIDKey = "request_id" // 保存在 Gin Context 中的 Request ID 键
)

// 构造 Request ID 中间件
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 生成 Request ID
		rid := strings.TrimSpace(c.GetHeader(HeaderRequestID))
		if rid == "" {
			rid = "req_" + uuid.NewString()
		}

		// 存进 Gin Context
		c.Set(ctxRequestIDKey, rid)
		// 写入响应头
		c.Writer.Header().Set(HeaderRequestID, rid)
		c.Next()
	}
}

// 从 Gin Context 中获取 Request ID 的函数
func GetRequestID(c *gin.Context) string {
	if v, ok := c.Get(ctxRequestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
