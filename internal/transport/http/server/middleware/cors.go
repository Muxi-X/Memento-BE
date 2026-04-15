package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 跨域请求处理中间件

type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string // 可显示在前端的响应头
	AllowCredentials bool
}

func CORS(cfg CORSConfig) gin.HandlerFunc {
	// 加载配置，没有配置时使用默认值
	allowAll := len(cfg.AllowOrigins) == 0
	allowMethods := strings.Join(defaultIfEmpty(cfg.AllowMethods, []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}), ", ")
	allowHeaders := strings.Join(defaultIfEmpty(cfg.AllowHeaders, []string{
		"Authorization",
		"Content-Type",
		"X-Request-ID",
		"Idempotency-Key",
		"X-Idempotency-Key",
	}), ", ")
	exposeHeaders := strings.Join(defaultIfEmpty(cfg.ExposeHeaders, []string{HeaderRequestID}), ", ")

	// 用于确认 Origin 是否在允许列表中，构建一个 map
	allowed := map[string]struct{}{}
	for _, o := range cfg.AllowOrigins {
		allowed[strings.TrimSpace(o)] = struct{}{}
	}

	return func(c *gin.Context) {
		// 获取 Origin 头
		origin := c.GetHeader("Origin")
		if origin != "" {
			// 若 allowAll 为 true，则允许所有 Origin
			if allowAll {
				c.Header("Access-Control-Allow-Origin", origin)
			} else if _, ok := allowed[origin]; ok {
				// 否则检查 Origin 是否在允许列表中
				c.Header("Access-Control-Allow-Origin", origin)
			}
		}

		// 写入其他 CORS 相关的响应头
		c.Header("Access-Control-Allow-Methods", allowMethods)
		c.Header("Access-Control-Allow-Headers", allowHeaders)
		c.Header("Access-Control-Expose-Headers", exposeHeaders)
		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		// 如果是预检请求，Abort 并返回 204 No Content
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// 没传配置时启用默认值
func defaultIfEmpty(v, def []string) []string {
	if len(v) == 0 {
		return def
	}
	return v
}
