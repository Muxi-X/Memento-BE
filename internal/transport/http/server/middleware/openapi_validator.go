package middleware

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	oapimw "github.com/oapi-codegen/gin-middleware"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
)

// 将第三方 OpenAPI 校验器的初始化封装成中间件构造函数

// 接收 OpenAPI 文档、认证校验函数和错误处理函数，返回 Gin 请求校验中间件
func NewOpenAPIRequestValidator(swagger *openapi3.T, auth openapi3filter.AuthenticationFunc, errHandler oapimw.ErrorHandler) (gin.HandlerFunc, error) {
	if swagger == nil {
		return nil, fmt.Errorf("swagger is nil")
	}

	// 校验 OpenAPI 文档是否有效
	if err := swagger.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("invalid openapi spec: %w", err)
	}

	// 配置
	opts := &oapimw.Options{
		ErrorHandler: errHandler,
		Options: openapi3filter.Options{
			AuthenticationFunc: auth,
		},
		SilenceServersWarning: true, // OpenAPI 文档里 servers 定义和当前请求不匹配时不警告
	}

	// 返回 Gin 请求校验中间件
	return oapimw.OapiRequestValidatorWithOptions(swagger, opts), nil
}
