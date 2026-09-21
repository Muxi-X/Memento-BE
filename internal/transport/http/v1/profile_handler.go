package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"

	profileapp "cixing/internal/modules/profile/application"
	"cixing/internal/transport/http/server/response"
	v1gen "cixing/internal/transport/http/v1/gen"
)

// (GET /v1/me/settings)
func (h *Handler) GetMeSettings(c *gin.Context, _ v1gen.GetMeSettingsParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.Profile.GetSettings(c.Request.Context(), userID)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, meSettingsResponse(*out))
}

// (PATCH /v1/me/settings/notifications)
func (h *Handler) UpdateMeNotificationSettings(c *gin.Context, _ v1gen.UpdateMeNotificationSettingsParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	var req v1gen.UpdateMeNotificationSettingsRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.Profile.UpdateNotificationSettings(c.Request.Context(), userID, profileapp.NotificationSettingsPatch{
		ReactionEnabled:         req.ReactionEnabled,
		CreationReminderEnabled: req.CreationReminderEnabled,
	})
	if err != nil {
		writeProfileError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, meSettingsResponse(*out))
}

// (PATCH /v1/me/profile/nickname)
func (h *Handler) UpdateMeNickname(c *gin.Context, _ v1gen.UpdateMeNicknameParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	var req v1gen.UpdateMeNicknameRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.Profile.UpdateNickname(c.Request.Context(), userID, req.Nickname)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, meProfileResponse(*out))
}
