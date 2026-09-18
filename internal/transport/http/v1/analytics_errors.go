package v1

import (
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"

	analyticsapp "cixing/internal/modules/analytics/application"
	"cixing/internal/transport/http/server/response"
)

func writeAnalyticsError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, analyticsapp.ErrUnauthenticatedUser):
		writeUnauthorized(c)
	default:
		var validationErr *analyticsapp.ValidationError
		if errors.As(err, &validationErr) {
			details := make([]response.FieldError, 0, len(validationErr.Fields))
			for _, field := range validationErr.Fields {
				name := field.Field
				if field.EventIndex >= 0 {
					name = fmt.Sprintf("events[%d].%s", field.EventIndex, field.Field)
				}
				details = append(details, response.FieldError{
					Field:  name,
					Rule:   field.Rule,
					Reason: field.Reason,
				})
			}
			writeValidationFields(c, details)
			return
		}
		writeInternal(c, err)
	}
}
