package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	danalytics "cixing/internal/modules/analytics/domain"
	"cixing/internal/transport/http/server/middleware"
	"cixing/internal/transport/http/server/response"
	v1gen "cixing/internal/transport/http/v1/gen"
)

// (POST /v1/analytics/events/batch)
func (h *Handler) BatchAnalyticsEvents(c *gin.Context, _ v1gen.BatchAnalyticsEventsParams) {
	userID, ok := userIDFromContext(c)
	if !ok {
		writeUnauthorized(c)
		return
	}

	var req v1gen.AnalyticsEventsBatchRequest
	if !bindJSON(c, &req) {
		return
	}

	events := make([]danalytics.Event, 0, len(req.Events))
	for _, event := range req.Events {
		var kind *string
		if event.Kind != nil {
			value := string(*event.Kind)
			kind = &value
		}
		var contextBizDate *time.Time
		if event.ContextBizDate != nil {
			value := event.ContextBizDate.Time
			contextBizDate = &value
		}
		var keywordID *uuid.UUID
		if event.KeywordId != nil {
			value := uuid.UUID(*event.KeywordId)
			keywordID = &value
		}
		events = append(events, danalytics.Event{
			EventID:        uuid.UUID(event.EventId),
			EventName:      danalytics.EventName(event.EventName),
			Kind:           kind,
			Source:         string(event.Source),
			SchemaVersion:  int(event.SchemaVersion),
			OccurredAt:     event.OccurredAt,
			SelectionState: danalytics.SelectionState(event.SelectionState),
			ContextBizDate: contextBizDate,
			KeywordID:      keywordID,
		})
	}

	acknowledged, err := h.Analytics.RecordBatch(c.Request.Context(), userID, middleware.GetRequestID(c), events)
	if err != nil {
		writeAnalyticsError(c, err)
		return
	}

	ids := make([]openapi_types.UUID, 0, len(acknowledged))
	for _, id := range acknowledged {
		ids = append(ids, openapi_types.UUID(id))
	}
	response.JSON(c, http.StatusOK, v1gen.AnalyticsEventsBatchResponse{AcknowledgedEventIds: ids})
}
