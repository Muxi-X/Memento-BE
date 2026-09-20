package domain

import (
	"time"

	"github.com/google/uuid"
)

type OfficialKeyword struct {
	ID           uuid.UUID
	Text         string
	Category     KeywordCategory
	IsActive     bool
	DisplayOrder int32
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type OfficialPrompt struct {
	ID           uuid.UUID
	KeywordID    uuid.UUID
	Kind         PromptKind
	Content      string
	DisplayOrder int32
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type DailyKeywordAssignment struct {
	BizDate   time.Time
	KeywordID uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

type DailyKeywordStat struct {
	BizDate              time.Time
	ParticipantUserCount int32
	UploadCount          int32
	ImageCount           int32
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type KeywordWithStats struct {
	BizDate              time.Time
	Keyword              OfficialKeyword
	ParticipantUserCount int32
	UploadCount          int32
	ImageCount           int32
}

type DailyPrompt struct {
	UserID          uuid.UUID
	BizDate         time.Time
	KeywordID       uuid.UUID
	PromptID        uuid.UUID
	Kind            PromptKind
	ContentSnapshot string
	SelectedAt      time.Time
}

type RotationState struct {
	Initialized       bool
	EffectiveDate     time.Time
	LastPlannedDate   time.Time
	NextQueuePosition int64
}

type RotationQueueKeyword struct {
	KeywordID     uuid.UUID
	QueuePosition int64
	IsActive      bool
}

type KeywordChangeAction string

const (
	KeywordChangeActivate   KeywordChangeAction = "activate"
	KeywordChangeDeactivate KeywordChangeAction = "deactivate"
)

type OfficialKeywordChange struct {
	ID            int64
	KeywordID     uuid.UUID
	Action        KeywordChangeAction
	EffectiveDate time.Time
	AppliedAt     time.Time
	CreatedAt     time.Time
}

type InsertOfficialKeywordChangeParams struct {
	KeywordID     uuid.UUID
	Action        KeywordChangeAction
	EffectiveDate time.Time
}
