package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
)

// 认证中间件
// 主要功能是从 Bearer Token 里识别当前用户，并把 user_id 放到 Gin Context 中

// 该接口用于验证 token 并提取用户 ID
type AccessTokenVerifier interface {
	VerifyAccessToken(ctx context.Context, token string) (userID string, err error)
}

// 登录可选，使用
func AuthOptional(verifier AccessTokenVerifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 若请求头中没有 Authorization，放行请求
		token := bearerToken(c.GetHeader("Authorization"))
		if token == "" {
			c.Next()
			return
		}

		// 有 token，验证成功后取 user_id 放到 Gin Context 中
		// token 验证失败时放行，由 OpenAPI validator 根据接口定义决定是否拒绝请求
		userID, err := verifier.VerifyAccessToken(c.Request.Context(), token)
		if err == nil && userID != "" {
			c.Set(CtxUserIDKey, userID)
		}
		c.Next()
	}
}

// 取 Token
func bearerToken(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return ""
	}
	const prefix = "bearer "
	if len(h) < len(prefix) {
		return ""
	}
	if strings.ToLower(h[:len(prefix)]) != prefix {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}
