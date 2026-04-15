package server

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/gin-gonic/gin"
	oapimw "github.com/oapi-codegen/gin-middleware"

	platformlogging "cixing/internal/platform/logging"
	"cixing/internal/transport/http/server/middleware"
	"cixing/internal/transport/http/server/response"
	v1 "cixing/internal/transport/http/v1"
	v1gen "cixing/internal/transport/http/v1/gen"
)

type Options struct {
	Logger  *slog.Logger
	V1      v1gen.ServerInterface
	GinMode string

	RateLimitRPS   int
	RateLimitBurst int

	AccessTokenVerifier middleware.AccessTokenVerifier // 校验 Token 的接口
}

func NewRouter(opts Options) *gin.Engine {
	if opts.Logger == nil {
		opts.Logger = platformlogging.NewLogger(platformlogging.LoggerConfig{Service: "cixing-api"})
	}
	if opts.GinMode != "" {
		gin.SetMode(opts.GinMode)
	}

	r := gin.New()
	r.Use(middleware.RequestID())
	r.Use(middleware.Recover(opts.Logger))
	r.Use(middleware.AccessLog(opts.Logger))
	r.Use(middleware.CORS(middleware.CORSConfig{})) // 不传 config 时使用默认配置，在 middleware 包

	r.Use(middleware.RateLimit(middleware.RateLimitConfig{
		RPS:   opts.RateLimitRPS,
		Burst: opts.RateLimitBurst,
	}))

	if opts.AccessTokenVerifier != nil {
		r.Use(middleware.AuthOptional(opts.AccessTokenVerifier))
	}

	// 注册 v1 路由，启用 v1 的 OpenAPI validator
	if opts.V1 != nil {
		// 创建 OpenAPI 请求校验中间件，OpenAPIRequestValidator
		// 当请求不符合 OpenAPI 文档要求时，拦下请求，调用 errHandler 写统一错误响应

		// GetSwagger() 从 OpenAPI 生成代码里取出 Swagger/OpenAPI 文档对象
		// 用于 OpenAPIRequestValidator 校验请求
		if swagger, err := v1gen.GetSwagger(); err == nil && swagger != nil {
			// 当接口文档里的某个接口声明了 bearerAuth
			// validator 会调用 authFunc 校验认证
			authFunc := func(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
				if input == nil {
					return nil
				}

				switch input.SecuritySchemeName {
				case "bearerAuth":
					// 检测 user_id 是否已被前面的中间件放进 Gin Context
					if userID := getUserIDFromGinContext(ctx); userID != "" {
						return nil
					}
					req := input.RequestValidationInput
					if req == nil || req.Request == nil {
						return input.NewError(errors.New("unauthorized: missing request"))
					}
					// 如果没有 user_id，从 Authorization 头里拆 Bearer Token
					token := bearerToken(req.Request.Header.Get("Authorization"))
					if token == "" {
						return input.NewError(errors.New("unauthorized: missing bearer token"))
					}
					// 获得 user_id
					if opts.AccessTokenVerifier == nil {
						return input.NewError(errors.New("unauthorized: verifier not configured"))
					}
					userID, err := opts.AccessTokenVerifier.VerifyAccessToken(req.Request.Context(), token)
					if err != nil || userID == "" {
						return input.NewError(errors.New("unauthorized: invalid bearer token"))
					}
					// 将 user_id 放进 Gin Context
					setUserIDOnGinContext(ctx, userID)
					return nil
				default:
					return nil
				}
			}

			// validator 校验失败时统一错误出口
			errHandler := func(c *gin.Context, message string, statusCode int) {
				// security 相关错误，映射成 401/403 错误体
				if isOpenAPISecurityError(message) {
					if strings.Contains(message, openAPISecurityForbiddenMarker) {
						response.Write(c, response.Err(response.Forbidden, "common.forbidden", "forbidden", nil))
						return
					}
					if strings.Contains(message, openAPISecurityUnauthorizedMarker) {
						response.Write(c, response.Err(response.Unauthorized, "common.unauthorized", "unauthorized", nil))
						return
					}
					response.Write(c, response.Err(response.Unauthorized, "common.unauthorized", "unauthorized", nil))
					return
				}
				// 其他错误，走 response.FromGinServerOptions
				response.FromGinServerOptions(c, errors.New(message), statusCode)
			}

			// 创建 OpenAPI 请求校验中间件
			if mw, err := middleware.NewOpenAPIRequestValidator(swagger, authFunc, errHandler); err == nil {
				r.Use(mw)
			} else {
				opts.Logger.Error("openapi validator init failed", slog.String("err", err.Error()))
			}
		} else if err != nil {
			opts.Logger.Error("openapi swagger load failed", slog.String("err", err.Error()))
		}

		// 注册 v1 路由
		v1.Register(r, opts.V1)
	}

	return r
}

// 将 Authorization 头里的 Bearer Token 拆成纯 Token 字符串
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

// 从 Gin Context 里取出 user_id
func getUserIDFromGinContext(ctx context.Context) string {
	// 获得 gin.Context
	ginCtx := oapimw.GetGinContext(ctx)
	if ginCtx == nil {
		return ""
	}
	if v, ok := ginCtx.Get(middleware.CtxUserIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// 将 user_id 放进 Gin Context
func setUserIDOnGinContext(ctx context.Context, userID string) {
	ginCtx := oapimw.GetGinContext(ctx)
	if ginCtx == nil {
		return
	}
	ginCtx.Set(middleware.CtxUserIDKey, userID)
}

const (
	openAPISecurityErrorMarker        = "SecurityRequirementsError"
	openAPISecurityUnauthorizedMarker = "unauthorized:"
	openAPISecurityForbiddenMarker    = "forbidden:"
)

func isOpenAPISecurityError(message string) bool {
	return strings.Contains(message, openAPISecurityErrorMarker)
}
