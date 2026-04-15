package middleware

import (
	"log/slog"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

// 处理请求处理中发生的 panic 的中间件

func Recover(l *slog.Logger) gin.HandlerFunc {
	if l == nil {
		l = slog.Default()
	}

	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				l.Error("panic",
					slog.Any("error", r),
					slog.String("request_id", GetRequestID(c)),
					slog.String("path", c.Request.URL.Path),
					slog.String("method", c.Request.Method),
					slog.String("stack", string(debug.Stack())), // debug.Stack() 获取当前 goroutine 的堆栈信息
				)

				// panic 时返回的响应
				c.AbortWithStatusJSON(500, gin.H{
					"code":       "internal",
					"reason":     "common.internal",
					"message":    "internal error",
					"request_id": GetRequestID(c),
				})
			}
		}()

		c.Next()
	}
}
