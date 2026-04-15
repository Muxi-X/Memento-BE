package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	publishingapp "cixing/internal/modules/publishing/application"
	dpub "cixing/internal/modules/publishing/domain"
	"cixing/internal/transport/http/server/response"
	v1gen "cixing/internal/transport/http/v1/gen"
)

// (POST /v1/uploads/publish-sessions)
func (h *Handler) CreateUploadPublishSession(c *gin.Context, _ v1gen.CreateUploadPublishSessionParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}

	var req v1gen.CreateUploadPublishSessionRequest
	if !bindJSON(c, &req) {
		return
	}

	var (
		contextType       dpub.PublishContextType
		officialKeywordID *uuid.UUID
		customKeywordID   *uuid.UUID
		bizDate           *time.Time
	)
	switch req.Context.Type {
	case v1gen.OfficialToday:
		details := make([]response.FieldError, 0, 3)
		if req.Context.OfficialKeywordId == nil {
			details = append(details, response.FieldError{
				Field:  "context.official_keyword_id",
				Rule:   "required",
				Reason: "is required when context.type is official_today",
			})
		}
		if req.Context.BizDate == nil {
			details = append(details, response.FieldError{
				Field:  "context.biz_date",
				Rule:   "required",
				Reason: "is required when context.type is official_today",
			})
		}
		if req.Context.CustomKeywordId != nil {
			details = append(details, response.FieldError{
				Field:  "context.custom_keyword_id",
				Rule:   "forbidden",
				Reason: "must be empty when context.type is official_today",
			})
		}
		if len(details) > 0 {
			writeValidationFields(c, details)
			return
		}
		contextType = dpub.PublishContextOfficialToday
		officialKeywordID = openapiUUIDPtrToUUIDPtr(req.Context.OfficialKeywordId)
		bizDate = openapiDatePtrToTimePtr(req.Context.BizDate)
	case v1gen.CustomKeyword:
		details := make([]response.FieldError, 0, 3)
		if req.Context.CustomKeywordId == nil {
			details = append(details, response.FieldError{
				Field:  "context.custom_keyword_id",
				Rule:   "required",
				Reason: "is required when context.type is custom_keyword",
			})
		}
		if req.Context.OfficialKeywordId != nil {
			details = append(details, response.FieldError{
				Field:  "context.official_keyword_id",
				Rule:   "forbidden",
				Reason: "must be empty when context.type is custom_keyword",
			})
		}
		if req.Context.BizDate != nil {
			details = append(details, response.FieldError{
				Field:  "context.biz_date",
				Rule:   "forbidden",
				Reason: "must be empty when context.type is custom_keyword",
			})
		}
		if len(details) > 0 {
			writeValidationFields(c, details)
			return
		}
		contextType = dpub.PublishContextCustomKeyword
		customKeywordID = openapiUUIDPtrToUUIDPtr(req.Context.CustomKeywordId)
	default:
		writeFieldValidation(c, "context.type", "oneof", "must be one of official_today, custom_keyword")
		return
	}

	out, err := h.PublishingSessions.Create(c.Request.Context(), publishingapp.CreateUploadSessionInput{
		OwnerUserID:       userID,
		ContextType:       contextType,
		OfficialKeywordID: officialKeywordID,
		CustomKeywordID:   customKeywordID,
		BizDate:           bizDate,
	})
	if err != nil {
		writeUploadPublishError(c, err)
		return
	}

	response.JSON(c, http.StatusCreated, v1gen.CreateUploadPublishSessionResponse{
		SessionId: openapi_types.UUID(out.SessionID),
		Status:    v1gen.UploadPublishSessionStatus(out.Status),
		ExpiresAt: out.ExpiresAt,
	})
}

// (POST /v1/uploads/publish-sessions/{session_id}/assets/complete-batch)
func (h *Handler) CompleteUploadPublishSessionAssetsBatch(c *gin.Context, sessionID openapi_types.UUID, _ v1gen.CompleteUploadPublishSessionAssetsBatchParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}

	var req v1gen.UploadPublishCompleteBatchRequest
	if !bindJSON(c, &req) {
		return
	}

	items := make([]publishingapp.CompleteBatchItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, publishingapp.CompleteBatchItemInput{
			ItemID:          uuid.UUID(item.ItemId),
			ImageEtag:       item.ImageEtag,
			ImageWidth:      item.ImageWidth,
			ImageHeight:     item.ImageHeight,
			AudioEtag:       item.AudioEtag,
			AudioDurationMS: item.AudioDurationMs,
			DisplayOrder:    item.DisplayOrder,
			IsCover:         item.IsCover != nil && *item.IsCover,
			Title:           item.Title,
			Note:            item.Note,
		})
	}

	out, err := h.PublishingSessions.CompleteBatch(c.Request.Context(), publishingapp.CompleteBatchInput{
		OwnerUserID: userID,
		SessionID:   uuid.UUID(sessionID),
		Items:       items,
	})
	if err != nil {
		writeUploadPublishError(c, err)
		return
	}

	respItems := make([]v1gen.UploadPublishCompleteBatchItemResponse, 0, len(out.Items))
	for _, item := range out.Items {
		respItems = append(respItems, v1gen.UploadPublishCompleteBatchItemResponse{
			ItemId:       openapi_types.UUID(item.ItemID),
			DisplayOrder: item.DisplayOrder,
			IsCover:      item.IsCover,
			Status:       v1gen.UploadPublishSessionItemStatus(item.Status),
		})
	}
	response.JSON(c, http.StatusOK, v1gen.UploadPublishCompleteBatchResponse{
		SessionId: openapi_types.UUID(out.SessionID),
		Status:    v1gen.UploadPublishSessionStatus(out.Status),
		Items:     respItems,
	})
}

// (POST /v1/uploads/publish-sessions/{session_id}/assets/presign-batch)
func (h *Handler) PresignUploadPublishSessionAssetsBatch(c *gin.Context, sessionID openapi_types.UUID, _ v1gen.PresignUploadPublishSessionAssetsBatchParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}

	var req v1gen.UploadPublishPresignBatchRequest
	if !bindJSON(c, &req) {
		return
	}

	items := make([]publishingapp.PresignBatchItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, publishingapp.PresignBatchItemInput{
			ClientImageID:      item.ClientImageId,
			ImageContentType:   item.ImageContentType,
			ImageContentLength: item.ImageContentLength,
			ImageSHA256:        item.ImageSha256,
			AudioContentType:   item.AudioContentType,
			AudioContentLength: item.AudioContentLength,
			AudioSHA256:        item.AudioSha256,
		})
	}

	out, err := h.PublishingSessions.PresignBatch(c.Request.Context(), publishingapp.PresignBatchInput{
		OwnerUserID: userID,
		SessionID:   uuid.UUID(sessionID),
		Items:       items,
	})
	if err != nil {
		writeUploadPublishError(c, err)
		return
	}

	respItems := make([]v1gen.UploadPublishPresignBatchItemResponse, 0, len(out.Items))
	for _, item := range out.Items {
		respItem := v1gen.UploadPublishPresignBatchItemResponse{
			ItemId:        openapi_types.UUID(item.ItemID),
			ClientImageId: item.ClientImageID,
			ImageId:       openapi_types.UUID(item.ImageID),
			ImageUpload:   uploadPresignedTargetToDTO(item.ImageUpload),
		}
		if item.AudioUpload != nil {
			audioUpload := uploadPresignedTargetToDTO(*item.AudioUpload)
			respItem.AudioUpload = &audioUpload
		}
		respItems = append(respItems, respItem)
	}
	response.JSON(c, http.StatusOK, v1gen.UploadPublishPresignBatchResponse{
		SessionId: openapi_types.UUID(out.SessionID),
		Items:     respItems,
	})
}

// (POST /v1/uploads/publish-sessions/{session_id}/commit)
func (h *Handler) CommitUploadPublishSession(c *gin.Context, sessionID openapi_types.UUID, _ v1gen.CommitUploadPublishSessionParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}
	out, err := h.PublishingSessions.Commit(c.Request.Context(), publishingapp.CommitUploadSessionInput{
		OwnerUserID: userID,
		SessionID:   uuid.UUID(sessionID),
	})
	if err != nil {
		writeUploadPublishError(c, err)
		return
	}
	response.JSON(c, http.StatusOK, v1gen.UploadPublishCommitResponse{
		SessionId:        openapi_types.UUID(out.SessionID),
		Status:           v1gen.UploadPublishSessionStatus(out.Status),
		UploadId:         openapi_types.UUID(out.UploadID),
		UploadVisibility: v1gen.UploadPublishVisibilityStatus(out.UploadVisibility),
	})
}

func uploadPresignedTargetToDTO(in publishingapp.UploadPresignedTarget) v1gen.UploadPresignedTarget {
	return v1gen.UploadPresignedTarget{
		Method:    v1gen.UploadPresignedTargetMethod(in.Method),
		Url:       in.URL,
		Headers:   stringMapPtr(in.Headers),
		ObjectKey: in.ObjectKey,
		ExpiresAt: in.ExpiresAt,
	}
}

func openapiUUIDPtrToUUIDPtr(v *openapi_types.UUID) *uuid.UUID {
	if v == nil {
		return nil
	}
	id := uuid.UUID(*v)
	return &id
}

func openapiDatePtrToTimePtr(v *openapi_types.Date) *time.Time {
	if v == nil {
		return nil
	}
	t := v.Time
	return &t
}

func stringMapPtr(src map[string]string) *map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return &dst
}
