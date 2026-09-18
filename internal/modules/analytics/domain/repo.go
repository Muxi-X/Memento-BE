package domain

import (
	"context"
	"time"
)

type Repository interface {
	InsertEvent(ctx context.Context, params InsertEventParams) error
	GetClickMetrics(ctx context.Context, bizDate time.Time) ([]ClickMetric, error)
	GetDailyPromptSuccessCount(ctx context.Context, bizDate time.Time) (int64, error)
}
