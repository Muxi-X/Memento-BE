package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	customapp "cixing/internal/modules/customkeywords/application"
	"cixing/internal/transport/http/server/response"
	v1gen "cixing/internal/transport/http/v1/gen"
)

// (GET /v1/custom-keywords)
func (h *Handler) ListCustomKeywords(c *gin.Context, _ v1gen.ListCustomKeywordsParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.CustomKeywords.List(c.Request.Context(), userID)
	if err != nil {
		writeCustomKeywordError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, customKeywordListResponse(out))
}

// (POST /v1/custom-keywords)
func (h *Handler) CreateCustomKeyword(c *gin.Context, _ v1gen.CreateCustomKeywordParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	var req v1gen.CreateCustomKeywordRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.CustomKeywords.Create(c.Request.Context(), userID, req.Text, req.TargetImageCount)
	if err != nil {
		writeCustomKeywordError(c, err)
		return
	}
	response.JSON(c, http.StatusCreated, customKeywordItemResponse(*out))
}

// (DELETE /v1/custom-keywords/{keyword_id})
func (h *Handler) DeleteCustomKeyword(c *gin.Context, keywordID openapi_types.UUID, _ v1gen.DeleteCustomKeywordParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	if err := h.CustomKeywords.Delete(c.Request.Context(), userID, uuid.UUID(keywordID)); err != nil {
		writeCustomKeywordError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// (PATCH /v1/custom-keywords/{keyword_id})
func (h *Handler) UpdateCustomKeyword(c *gin.Context, keywordID openapi_types.UUID, _ v1gen.UpdateCustomKeywordParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	var req v1gen.UpdateCustomKeywordRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.CustomKeywords.Update(c.Request.Context(), userID, uuid.UUID(keywordID), customapp.UpdateKeywordInput{
		Text:             req.Text,
		TargetImageCount: req.TargetImageCount,
		IsActive:         req.IsActive,
	})
	if err != nil {
		writeCustomKeywordError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, customKeywordItemResponse(*out))
}

// (DELETE /v1/custom-keywords/{keyword_id}/cover)
func (h *Handler) ClearCustomKeywordCover(c *gin.Context, keywordID openapi_types.UUID, _ v1gen.ClearCustomKeywordCoverParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.CustomKeywords.ClearCover(c.Request.Context(), userID, uuid.UUID(keywordID))
	if err != nil {
		writeCustomKeywordError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, customKeywordCoverResponse(*out))
}

// (PATCH /v1/custom-keywords/{keyword_id}/cover)
func (h *Handler) SetCustomKeywordCover(c *gin.Context, keywordID openapi_types.UUID, _ v1gen.SetCustomKeywordCoverParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	var req v1gen.SetCustomKeywordCoverRequest
	if !bindJSON(c, &req) {
		return
	}
	out, err := h.CustomKeywords.SetCover(c.Request.Context(), userID, uuid.UUID(keywordID), uuid.UUID(req.ImageId))
	if err != nil {
		writeCustomKeywordError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, customKeywordCoverResponse(*out))
}

// (GET /v1/custom-keywords/{keyword_id}/images)
func (h *Handler) ListCustomKeywordImages(c *gin.Context, keywordID openapi_types.UUID, params v1gen.ListCustomKeywordImagesParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.CustomKeywords.ListImages(c.Request.Context(), userID, uuid.UUID(keywordID), ptrIntValue(params.Limit))
	if err != nil {
		writeCustomKeywordError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, customKeywordImagesResponse(*out))
}

// (GET /v1/custom/images/{image_id})
func (h *Handler) GetCustomImage(c *gin.Context, imageID openapi_types.UUID, _ v1gen.GetCustomImageParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.CustomKeywords.GetImage(c.Request.Context(), userID, uuid.UUID(imageID))
	if err != nil {
		writeCustomKeywordError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, customKeywordImageDetailResponse(*out))
}
