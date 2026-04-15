package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"

	v1gen "cixing/internal/transport/http/v1/gen"
)

// (GET /healthz)
func (h *Handler) Healthz(c *gin.Context, _ v1gen.HealthzParams) {
	c.Status(http.StatusOK)
}
