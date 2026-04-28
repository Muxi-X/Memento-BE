package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	profileapp "cixing/internal/modules/profile/application"
	"cixing/internal/transport/http/server/response"
	v1gen "cixing/internal/transport/http/v1/gen"
)

// (POST /v1/me/avatar-upload-sessions)
func (h *Handler) CreateAvatarUploadSession(c *gin.Context, _ v1gen.CreateAvatarUploadSessionParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.AvatarUploads.Create(c.Request.Context(), userID)
	if err != nil {
		writeAvatarUploadError(c, err)
		return
	}
	response.JSON(c, http.StatusCreated, v1gen.CreateAvatarUploadSessionResponse{
		SessionId: openapi_types.UUID(out.SessionID),
		Status:    v1gen.AvatarUploadSessionStatus(out.Status),
		ExpiresAt: out.ExpiresAt,
	})
}

// (POST /v1/me/avatar-upload-sessions/{session_id}/image/presign)
func (h *Handler) PresignAvatarUploadSessionImage(c *gin.Context, sessionID openapi_types.UUID, _ v1gen.PresignAvatarUploadSessionImageParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	var req v1gen.PresignAvatarImageRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.AvatarUploads.PresignImage(c.Request.Context(), profileapp.PresignAvatarImageInput{
		UserID:             userID,
		SessionID:          uuid.UUID(sessionID),
		ImageContentType:   req.ImageContentType,
		ImageContentLength: req.ImageContentLength,
		ImageSHA256:        req.ImageSha256,
	})
	if err != nil {
		writeAvatarUploadError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, v1gen.PresignAvatarImageResponse{
		SessionId:   openapi_types.UUID(out.SessionID),
		Status:      v1gen.AvatarUploadSessionStatus(out.Status),
		ImageId:     openapi_types.UUID(out.ImageID),
		ImageUpload: avatarPresignedTargetToDTO(out.ImageUpload),
	})
}

// (POST /v1/me/avatar-upload-sessions/{session_id}/complete)
func (h *Handler) CompleteAvatarUploadSession(c *gin.Context, sessionID openapi_types.UUID, _ v1gen.CompleteAvatarUploadSessionParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	var req v1gen.CompleteAvatarUploadSessionRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.AvatarUploads.Complete(c.Request.Context(), profileapp.CompleteAvatarUploadInput{
		UserID:      userID,
		SessionID:   uuid.UUID(sessionID),
		ImageETag:   req.ImageEtag,
		ImageWidth:  req.ImageWidth,
		ImageHeight: req.ImageHeight,
	})
	if err != nil {
		writeAvatarUploadError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, v1gen.CompleteAvatarUploadSessionResponse{
		SessionId: openapi_types.UUID(out.SessionID),
		Status:    v1gen.AvatarUploadSessionStatus(out.Status),
		Profile:   meProfileResponse(out.Profile),
	})
}

func avatarPresignedTargetToDTO(in profileapp.PresignedUploadTarget) v1gen.UploadPresignedTarget {
	return v1gen.UploadPresignedTarget{
		Method:    v1gen.UploadPresignedTargetMethod(in.Method),
		Url:       in.URL,
		Headers:   stringMapPtr(in.Headers),
		ObjectKey: in.ObjectKey,
		ExpiresAt: in.ExpiresAt,
	}
}
