package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type UpsertKeywordParams struct {
	ID           uuid.UUID
	Text         string
	Category     KeywordCategory
	IsActive     bool
	DisplayOrder *int32
}

type UpsertPromptParams struct {
	ID           uuid.UUID
	KeywordID    uuid.UUID
	Kind         PromptKind
	Content      string
	DisplayOrder *int32
	IsActive     bool
}

type UpsertDailyKeywordStatParams struct {
	BizDate              time.Time
	ParticipantUserCount int32
	UploadCount          int32
	ImageCount           int32
}

type Repository interface {
	GetOfficialKeywordByID(ctx context.Context, id uuid.UUID) (OfficialKeyword, error)
	GetOfficialKeywordByText(ctx context.Context, text string) (OfficialKeyword, error)
	ListActiveOfficialKeywords(ctx context.Context) ([]OfficialKeyword, error)
	UpsertOfficialKeyword(ctx context.Context, params UpsertKeywordParams) (OfficialKeyword, error)
	DeactivateOfficialKeyword(ctx context.Context, id uuid.UUID) (OfficialKeyword, error)

	DrawRandomPrompt(ctx context.Context, keywordID uuid.UUID, kind PromptKind) (OfficialPrompt, error)
	ListPromptsByKeyword(ctx context.Context, keywordID uuid.UUID) ([]OfficialPrompt, error)
	UpsertPrompt(ctx context.Context, params UpsertPromptParams) (OfficialPrompt, error)

	GetDailyKeywordAssignment(ctx context.Context, bizDate time.Time) (DailyKeywordAssignment, error)
	UpsertDailyKeywordAssignment(ctx context.Context, bizDate time.Time, keywordID uuid.UUID) (DailyKeywordAssignment, error)
	GetKeywordForDateWithStats(ctx context.Context, bizDate time.Time) (KeywordWithStats, error)
	GetDailyKeywordStat(ctx context.Context, bizDate time.Time) (DailyKeywordStat, error)
	UpsertDailyKeywordStat(ctx context.Context, params UpsertDailyKeywordStatParams) (DailyKeywordStat, error)
	RecomputeDailyKeywordStatsFromUploads(ctx context.Context, bizDate time.Time) (DailyKeywordStat, error)
}
