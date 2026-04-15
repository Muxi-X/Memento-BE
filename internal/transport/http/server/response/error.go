package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"cixing/internal/transport/http/server/middleware"
)

// 统一错误出口，翻译错误码和错误信息，并返回给客户端

type Code string

// 错误码
const (
	Validation      Code = "validation"
	Unauthorized    Code = "unauthorized"
	Forbidden       Code = "forbidden"
	NotFound        Code = "not_found"
	Conflict        Code = "conflict"
	RateLimited     Code = "rate_limited"
	FeatureNotReady Code = "feature_not_ready"
	Internal        Code = "internal"
)

// Error 响应体，最终返回给前端
type Error struct {
	Code      Code   `json:"code"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Details   any    `json:"details,omitempty"`
}

// 字段级错误：用于请求字段错误校验
type FieldError struct {
	Field  string `json:"field"`
	Rule   string `json:"rule"`
	Reason string `json:"reason"`
}

// 项目内部流转的错误
// 把“内部错误语义”先标准化，再统一翻译成 HTTP 响应
type AppError struct {
	Code    Code
	Reason  string
	Message string
	Details any
}

// 让 AppError 实现 Go 的 error 接口，使其可当 error 使用
func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	return string(e.Code) + ": " + e.Message
}

// 构造 AppError 的函数
func Err(code Code, reason, message string, details any) error {
	return &AppError{Code: code, Reason: reason, Message: message, Details: details}
}

func ValidationErr(details []FieldError, message string) error {
	if message == "" {
		message = "validation failed"
	}
	return Err(Validation, "validation.failed", message, details)
}

func defaultReasonForCode(code Code) string {
	switch code {
	case Validation:
		return "validation.failed"
	case Unauthorized:
		return "common.unauthorized"
	case Forbidden:
		return "common.forbidden"
	case NotFound:
		return "common.not_found"
	case Conflict:
		return "common.conflict"
	case RateLimited:
		return "common.rate_limited"
	case FeatureNotReady:
		return "common.feature_not_ready"
	default:
		return "common.internal"
	}
}

// 错误码映射到 HTTP 状态码
func statusFromCode(code Code) int {
	switch code {
	case Validation:
		return http.StatusBadRequest
	case Unauthorized:
		return http.StatusUnauthorized
	case Forbidden:
		return http.StatusForbidden
	case NotFound:
		return http.StatusNotFound
	case Conflict:
		return http.StatusConflict
	case RateLimited:
		return http.StatusTooManyRequests
	case FeatureNotReady:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// 统一错误出口函数
func Write(c *gin.Context, err error) {
	// 获得 RequestID
	rid := middleware.GetRequestID(c)

	// 获取 AppError
	var ae *AppError
	if !errors.As(err, &ae) || ae == nil {
		ae = &AppError{
			Code:    Internal,
			Reason:  defaultReasonForCode(Internal),
			Message: "internal error",
		}
	}
	if ae.Reason == "" {
		ae.Reason = defaultReasonForCode(ae.Code)
	}

	// 构造响应 Error
	resp := Error{
		Code:      ae.Code,
		Reason:    ae.Reason,
		Message:   ae.Message,
		RequestID: rid,
	}
	if ae.Details != nil {
		resp.Details = ae.Details
	}

	JSON(c, statusFromCode(ae.Code), resp)
	c.Abort()
}

// 给 gin.Server.Errorhandler 使用，将 gin.Error 转换为 Error 并写入响应
func FromGinServerOptions(c *gin.Context, err error, status int) {
	if status == http.StatusBadRequest {
		Write(c, ValidationErr([]FieldError{{
			Field:  "request",
			Rule:   "invalid",
			Reason: err.Error(),
		}}, ""))
		return
	}

	code := Internal
	switch status {
	case http.StatusBadRequest:
		code = Validation
	case http.StatusUnauthorized:
		code = Unauthorized
	case http.StatusForbidden:
		code = Forbidden
	case http.StatusNotFound:
		code = NotFound
	case http.StatusConflict:
		code = Conflict
	case http.StatusTooManyRequests:
		code = RateLimited
	case http.StatusInternalServerError:
		code = Internal
	}

	Write(c, Err(code, defaultReasonForCode(code), http.StatusText(status), nil))
}
