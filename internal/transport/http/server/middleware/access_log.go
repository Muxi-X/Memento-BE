package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// 为每个 HTTP 请求记录访问日志的中间件
// 在每个 HTTP 请求结束后，统一记录一条访问日志

const (
	CtxUserIDKey = "user_id"
)

// Logger 用来写日志
func AccessLog(l *slog.Logger) gin.HandlerFunc {
	if l == nil {
		l = slog.Default()
	}

	return func(c *gin.Context) {
		// 记录请求开始时间
		start := time.Now()
		// 放行后续中间件和 handler
		c.Next()

		// 请求结束时计算耗时
		lat := time.Since(start)
		// 取 Request ID
		rid := GetRequestID(c)
		// 取用户 ID
		uid := ""
		if v, ok := c.Get(CtxUserIDKey); ok {
			if s, ok := v.(string); ok {
				uid = s
			}
		}

		// 返回 Gin 匹配的路由模板
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}

		// 记录访问日志
		l.Info("http_request",
			slog.String("request_id", rid),
			slog.String("user_id", uid),
			slog.Duration("latency", lat),
			slog.String("method", c.Request.Method),
			slog.String("path", route),
			slog.Int("status", c.Writer.Status()),
		)
	}
}

// 不带 Metrics 的 AccessLog
func Logger(l *slog.Logger) gin.HandlerFunc {
	return AccessLog(l)
}
