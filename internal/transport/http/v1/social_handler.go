package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	v1gen "cixing/internal/transport/http/v1/gen"
)

// (PUT /v1/reactions/uploads/{upload_id})
func (h *Handler) ReactToUpload(c *gin.Context, uploadID openapi_types.UUID, _ v1gen.ReactToUploadParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	var req v1gen.CreateReactionRequest
	if !bindJSON(c, &req) {
		return
	}
	if err := h.SocialReactions.React(c.Request.Context(), userID, uuid.UUID(uploadID), string(req.Type)); err != nil {
		writeSocialError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// (DELETE /v1/reactions/uploads/{upload_id}/{type})
func (h *Handler) UnreactToUpload(c *gin.Context, uploadID openapi_types.UUID, reactionType v1gen.ReactionType, _ v1gen.UnreactToUploadParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	if err := h.SocialReactions.Unreact(c.Request.Context(), userID, uuid.UUID(uploadID), string(reactionType)); err != nil {
		writeSocialError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
