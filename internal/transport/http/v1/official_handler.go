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
func (h *Handler) DrawOfficialPrompt(c *gin.Context, keywordID openapi_types.UUID, _ v1gen.DrawOfficialPromptParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}

	var req v1gen.DrawPromptRequest
	if !bindJSON(c, &req) {
		return
	}

	var requestedBizDate *time.Time
	if req.BizDate != nil {
		value := req.BizDate.Time
		requestedBizDate = &value
	}

	out, err := h.OfficialPrompts.Draw(c.Request.Context(), userID, uuid.UUID(keywordID), string(req.Kind), requestedBizDate)
	if err != nil {
		writeOfficialPromptError(c, err)
		return
	}

	response.JSON(c, http.StatusOK, v1gen.DrawPromptResponse{
		Id:         openapi_types.UUID(out.ID),
		Kind:       v1gen.PromptKind(out.Kind),
		Content:    out.Content,
		BizDate:    openapi_types.Date{Time: out.BizDate},
		KeywordId:  openapi_types.UUID(out.KeywordID),
		SelectedAt: out.SelectedAt,
		ResetsAt:   out.ResetsAt,
	})
}

// (GET /v1/me/daily-prompt)
func (h *Handler) GetMeDailyPrompt(c *gin.Context, _ v1gen.GetMeDailyPromptParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}

	out, err := h.OfficialPrompts.GetDailyPrompt(c.Request.Context(), userID)
	if err != nil {
		writeOfficialPromptError(c, err)
		return
	}

	var keywordID *openapi_types.UUID
	if out.KeywordID != nil {
		value := openapi_types.UUID(*out.KeywordID)
		keywordID = &value
	}

	var selection *v1gen.DailyPromptSelection
	if out.Selection != nil {
		selection = &v1gen.DailyPromptSelection{
			Id:         openapi_types.UUID(out.Selection.ID),
			Kind:       v1gen.PromptKind(out.Selection.Kind),
			Content:    out.Selection.Content,
			SelectedAt: out.Selection.SelectedAt,
		}
	}

	response.JSON(c, http.StatusOK, v1gen.DailyPromptResponse{
		BizDate:   openapi_types.Date{Time: out.BizDate},
		ResetsAt:  out.ResetsAt,
		KeywordId: keywordID,
		Selection: selection,
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
