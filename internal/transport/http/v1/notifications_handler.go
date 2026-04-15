package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"cixing/internal/transport/http/server/response"
	v1gen "cixing/internal/transport/http/v1/gen"
)

// (GET /v1/me/notifications)
func (h *Handler) ListMeNotifications(c *gin.Context, _ v1gen.ListMeNotificationsParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.Notifications.List(c.Request.Context(), userID)
	if err != nil {
		writeSocialError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, notificationsResponse(*out))
}

// (PATCH /v1/me/notifications/read)
func (h *Handler) MarkAllMeNotificationsRead(c *gin.Context, _ v1gen.MarkAllMeNotificationsReadParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	if err := h.Notifications.MarkAllRead(c.Request.Context(), userID); err != nil {
		writeSocialError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
