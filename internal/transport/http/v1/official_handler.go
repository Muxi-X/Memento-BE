package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"cixing/internal/transport/http/server/response"
	v1gen "cixing/internal/transport/http/v1/gen"
)

// (GET /v1/official/home)
func (h *Handler) GetOfficialHome(c *gin.Context, params v1gen.GetOfficialHomeParams) {
	var baseDate *time.Time
	if params.Date != nil {
		t := params.Date.Time
		baseDate = &t
	}
	out, err := h.ReadModel.GetOfficialHome(c.Request.Context(), baseDate)
	if err != nil {
		writeReadModelError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, v1gen.OfficialHomeResponse{
		Today:     todayKeywordResponse(out.Today),
		Yesterday: todayKeywordResponse(out.Yesterday),
	})
}

// (GET /v1/official/dates/{biz_date}/uploads)
func (h *Handler) ListOfficialDateUploads(c *gin.Context, bizDate openapi_types.Date, params v1gen.ListOfficialDateUploadsParams) {
	viewerID, _ := userIDFromContext(c)
	var viewer *uuid.UUID
	if viewerID != uuid.Nil {
		viewer = &viewerID
	}
	out, err := h.ReadModel.ListOfficialDateUploads(
		c.Request.Context(),
		bizDate.Time,
		stringValue(params.Sort),
		ptrIntValue(params.Limit),
		params.Seed,
		boolValue(params.IncludeReactionCounts),
		viewer,
	)
	if err != nil {
		writeReadModelError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, publicUploadListResponse(out))
}

// (POST /v1/official/keywords/{keyword_id}/prompts/draw)
func (h *Handler) DrawOfficialPrompt(c *gin.Context, _ openapi_types.UUID, _ v1gen.DrawOfficialPromptParams) {
	var req v1gen.DrawPromptRequest
	if !bindJSON(c, &req) {
		return
	}

	keywordID, err := uuid.Parse(c.Param("keyword_id"))
	if err != nil {
		writeFieldValidation(c, "keyword_id", "uuid", "must be a valid UUID")
		return
	}

	out, err := h.OfficialPrompts.Draw(c.Request.Context(), keywordID, string(req.Kind))
	if err != nil {
		writeOfficialPromptError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, v1gen.Prompt{
		Id:      openapi_types.UUID(out.ID),
		Kind:    v1gen.PromptKind(out.Kind),
		Content: out.Content,
	})
}

// (GET /v1/official/uploads/{upload_id})
func (h *Handler) GetOfficialUpload(c *gin.Context, uploadID openapi_types.UUID, params v1gen.GetOfficialUploadParams) {
	viewerID, _ := userIDFromContext(c)
	var viewer *uuid.UUID
	if viewerID != uuid.Nil {
		viewer = &viewerID
	}
	out, err := h.ReadModel.GetOfficialUpload(c.Request.Context(), uuid.UUID(uploadID), boolValue(params.IncludeReactionCounts), viewer)
	if err != nil {
		writeReadModelError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, publicUploadDetailResponse(out))
}
