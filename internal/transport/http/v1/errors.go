package v1

import (
	"github.com/gin-gonic/gin"

	"cixing/internal/transport/http/server/response"
)

func writeAppError(c *gin.Context, code response.Code, reason, message string, details any) {
	response.Write(c, response.Err(code, reason, message, details))
}

func writeUnauthorized(c *gin.Context) {
	writeAppError(c, response.Unauthorized, "common.unauthorized", "unauthorized", nil)
}

func writeValidationFields(c *gin.Context, details []response.FieldError) {
	response.Write(c, response.ValidationErr(details, ""))
}

func writeFieldValidation(c *gin.Context, field, rule, reason string) {
	writeValidationFields(c, []response.FieldError{{
		Field:  field,
		Rule:   rule,
		Reason: reason,
	}})
}

func writeInternal(c *gin.Context, _ error) {
	writeAppError(c, response.Internal, "common.internal", "internal error", nil)
}
