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

type InsertDailyPromptParams struct {
	UserID          uuid.UUID
	BizDate         time.Time
	KeywordID       uuid.UUID
	PromptID        uuid.UUID
	Kind            PromptKind
	ContentSnapshot string
	SelectedAt      time.Time
}

type Repository interface {
	GetOfficialKeywordByID(ctx context.Context, id uuid.UUID) (OfficialKeyword, error)
	GetOfficialKeywordByText(ctx context.Context, text string) (OfficialKeyword, error)
	ListOfficialKeywords(ctx context.Context) ([]OfficialKeyword, error)
	ListActiveOfficialKeywords(ctx context.Context) ([]OfficialKeyword, error)
	InsertOfficialKeyword(ctx context.Context, params UpsertKeywordParams) (OfficialKeyword, error)
	UpdateOfficialKeyword(ctx context.Context, params UpsertKeywordParams) (OfficialKeyword, error)

	DrawRandomPrompt(ctx context.Context, keywordID uuid.UUID, kind PromptKind) (OfficialPrompt, error)
	ListPromptsByKeyword(ctx context.Context, keywordID uuid.UUID) ([]OfficialPrompt, error)
	UpsertPrompt(ctx context.Context, params UpsertPromptParams) (OfficialPrompt, error)

	LockUser(ctx context.Context, userID uuid.UUID) error
	GetDailyPrompt(ctx context.Context, userID uuid.UUID, bizDate time.Time) (DailyPrompt, error)
	InsertDailyPrompt(ctx context.Context, params InsertDailyPromptParams) (DailyPrompt, error)

	GetDailyKeywordAssignment(ctx context.Context, bizDate time.Time) (DailyKeywordAssignment, error)
	GetLastDailyKeywordAssignmentBefore(ctx context.Context, bizDate time.Time) (DailyKeywordAssignment, error)
	HasDailyKeywordAssignmentOnOrAfter(ctx context.Context, bizDate time.Time) (bool, error)
	InsertDailyKeywordAssignment(ctx context.Context, bizDate time.Time, keywordID uuid.UUID) (DailyKeywordAssignment, bool, error)
	ListDailyKeywordAssignmentsBetween(ctx context.Context, startDate time.Time, endDate time.Time) ([]DailyKeywordAssignment, error)

	LockRotationState(ctx context.Context) (RotationState, error)
	InitializeRotation(ctx context.Context, effectiveDate time.Time) (RotationState, error)
	InsertRotationQueueItem(ctx context.Context, keywordID uuid.UUID, queuePosition int64) error
	SyncRotationQueueTail(ctx context.Context) (RotationState, error)
	ListRotationQueue(ctx context.Context) ([]RotationQueueKeyword, error)
	MoveRotationKeywordToTail(ctx context.Context, keywordID uuid.UUID) (int64, error)
	MoveOrInsertRotationKeywordAtTail(ctx context.Context, keywordID uuid.UUID) error
	AdvanceRotationDate(ctx context.Context, bizDate time.Time) (RotationState, error)

	InsertOfficialKeywordChange(ctx context.Context, params InsertOfficialKeywordChangeParams) (OfficialKeywordChange, error)
	SnapshotOfficialKeywordCatalog(ctx context.Context) error
	RegisterOfficialKeywordBaseline(ctx context.Context, keywordID uuid.UUID) error
	HasOfficialKeywordCatalogMismatch(ctx context.Context) (bool, error)
	ListPendingOfficialKeywordChangesThrough(ctx context.Context, bizDate time.Time) ([]OfficialKeywordChange, error)
	SetOfficialKeywordActive(ctx context.Context, keywordID uuid.UUID, isActive bool) error
	MarkOfficialKeywordChangeApplied(ctx context.Context, changeID int64) error

	GetKeywordForDateWithStats(ctx context.Context, bizDate time.Time) (KeywordWithStats, error)
	GetDailyKeywordStat(ctx context.Context, bizDate time.Time) (DailyKeywordStat, error)
	UpsertDailyKeywordStat(ctx context.Context, params UpsertDailyKeywordStatParams) (DailyKeywordStat, error)
	RecomputeDailyKeywordStatsFromUploads(ctx context.Context, bizDate time.Time) (DailyKeywordStat, error)
}
