package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"cixing/internal/transport/http/server/response"
	v1gen "cixing/internal/transport/http/v1/gen"
)

// (GET /v1/me/home)
func (h *Handler) GetMeHome(c *gin.Context, _ v1gen.GetMeHomeParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}

	out, err := h.ReadModel.GetMeHome(c.Request.Context(), userID)
	if err != nil {
		writeReadModelError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, meHomeResponse(out))
}

// (GET /v1/review/dates)
func (h *Handler) ListReviewDates(c *gin.Context, params v1gen.ListReviewDatesParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.ReadModel.ListReviewDates(c.Request.Context(), userID, ptrIntValue(params.Limit))
	if err != nil {
		writeReadModelError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, reviewDatesResponse(out))
}

// (GET /v1/review/dates/uploads/my)
func (h *Handler) ListReviewMyUploadsByDate(c *gin.Context, params v1gen.ListReviewMyUploadsByDateParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.ReadModel.ListReviewMyUploadsByDate(c.Request.Context(), userID, params.BizDate.Time, ptrIntValue(params.Limit))
	if err != nil {
		writeReadModelError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, reviewUploadListResponse(out))
}

// (GET /v1/review/keywords)
func (h *Handler) ListReviewKeywords(c *gin.Context, _ v1gen.ListReviewKeywordsParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.ReadModel.ListReviewKeywords(c.Request.Context(), userID)
	if err != nil {
		writeReadModelError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, reviewKeywordsResponse(out))
}

// (GET /v1/review/keywords/{keyword_id}/uploads/all)
func (h *Handler) ListReviewAllUploadsByKeyword(c *gin.Context, keywordID openapi_types.UUID, params v1gen.ListReviewAllUploadsByKeywordParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.ReadModel.ListReviewAllUploadsByKeyword(
		c.Request.Context(),
		userID,
		uuid.UUID(keywordID),
		stringValue(params.Sort),
		ptrIntValue(params.Limit),
		params.Seed,
		boolValue(params.IncludeReactionCounts),
	)
	if err != nil {
		writeReadModelError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, publicUploadListResponse(out))
}

// (GET /v1/review/keywords/{keyword_id}/uploads/my)
func (h *Handler) ListReviewMyUploadsByKeyword(c *gin.Context, keywordID openapi_types.UUID, params v1gen.ListReviewMyUploadsByKeywordParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.ReadModel.ListReviewMyUploadsByKeyword(c.Request.Context(), userID, uuid.UUID(keywordID), ptrIntValue(params.Limit))
	if err != nil {
		writeReadModelError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, reviewUploadListResponse(out))
}

// (GET /v1/review/uploads/{upload_id})
func (h *Handler) GetReviewUpload(c *gin.Context, uploadID openapi_types.UUID, _ v1gen.GetReviewUploadParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.ReadModel.GetReviewUpload(c.Request.Context(), userID, uuid.UUID(uploadID))
	if err != nil {
		writeReadModelError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, reviewUploadDetailResponse(out))
}
