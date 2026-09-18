package repo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	danalytics "cixing/internal/modules/analytics/domain"
	analyticsdb "cixing/internal/modules/analytics/infra/db/gen"
	"cixing/internal/shared/common"
)

type Repository struct {
	q analyticsdb.Querier
}

var _ danalytics.Repository = (*Repository)(nil)

func NewRepository(q analyticsdb.Querier) *Repository {
	return &Repository{q: q}
}

func (r *Repository) InsertEvent(ctx context.Context, params danalytics.InsertEventParams) error {
	return r.q.InsertAnalyticsEvent(ctx, analyticsdb.InsertAnalyticsEventParams{
		UserID:         params.UserID,
		EventID:        params.EventID,
		EventName:      string(params.EventName),
		Kind:           textOrNull(params.Kind),
		Source:         params.Source,
		SchemaVersion:  int16(params.SchemaVersion),
		OccurredAt:     timestamptz(params.OccurredAt),
		SelectionState: string(params.SelectionState),
		ContextBizDate: dateOrNull(params.ContextBizDate),
		KeywordID:      uuidOrNull(params.KeywordID),
		ReceivedAt:     timestamptz(params.ReceivedAt),
		BizDate:        dateOnly(params.BizDate),
		RequestID:      params.RequestID,
	})
}

func (r *Repository) GetClickMetrics(ctx context.Context, bizDate time.Time) ([]danalytics.ClickMetric, error) {
	rows, err := r.q.GetAnalyticsClickMetrics(ctx, dateOnly(bizDate))
	if err != nil {
		return nil, err
	}
	out := make([]danalytics.ClickMetric, 0, len(rows))
	for _, row := range rows {
		out = append(out, danalytics.ClickMetric{
			EventName:       danalytics.EventName(row.EventName),
			Kind:            row.Kind,
			ClickCount:      row.ClickCount,
			UniqueUserCount: row.UniqueUserCount,
		})
	}
	return out, nil
}

func (r *Repository) GetDailyPromptSuccessCount(ctx context.Context, bizDate time.Time) (int64, error) {
	return r.q.GetDailyPromptSuccessCount(ctx, dateOnly(bizDate))
}

func textOrNull(v *string) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *v, Valid: true}
}

func dateOrNull(v *time.Time) pgtype.Date {
	if v == nil {
		return pgtype.Date{}
	}
	return dateOnly(*v)
}

func uuidOrNull(v *uuid.UUID) pgtype.UUID {
	if v == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *v, Valid: true}
}

func timestamptz(v time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: v, Valid: true}
}

func dateOnly(v time.Time) pgtype.Date {
	return pgtype.Date{Time: common.NormalizeBizDate(v), Valid: true}
}
