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
