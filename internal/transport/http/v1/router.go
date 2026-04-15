package v1

import (
	"github.com/gin-gonic/gin"

	"cixing/internal/transport/http/server/response"
	v1gen "cixing/internal/transport/http/v1/gen"
)

// 把 v1 的业务 handler 注册成真正可用的 HTTP 路由
func Register(r gin.IRouter, handler v1gen.ServerInterface) {
	// RegisterHandlersWithOptions 是由 oapi-codegen 生成的路由注册函数
	// 根据 OpenAPI 文档里定义的路径和方法，把所有接口挂到 Gin 上
	// 可以解析参数，调用 handler
	v1gen.RegisterHandlersWithOptions(r, handler, v1gen.GinServerOptions{
		BaseURL: "", // OpenAPI 文档已有完整路径
		// ErrorHandler 处理 wrapper 解析 HTTP 参数时产生的错误
		ErrorHandler: func(c *gin.Context, err error, status int) {
			// 走统一错误响应格式
			response.FromGinServerOptions(c, err, status)
		},
	})
}
