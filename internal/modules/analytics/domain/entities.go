package domain

import (
	"time"

	"github.com/google/uuid"
)

type EventName string

const (
	EventNamePromptEntryClick     EventName = "prompt_entry_click"
	EventNamePromptKindEntryClick EventName = "prompt_kind_entry_click"
)

type SelectionState string

const (
	SelectionStateUnknown    SelectionState = "unknown"
	SelectionStateUnselected SelectionState = "unselected"
	SelectionStateSelected   SelectionState = "selected"
)

type Event struct {
	EventID        uuid.UUID
	EventName      EventName
	Kind           *string
	Source         string
	SchemaVersion  int
	OccurredAt     time.Time
	SelectionState SelectionState
	ContextBizDate *time.Time
	KeywordID      *uuid.UUID
}

type InsertEventParams struct {
	UserID         uuid.UUID
	EventID        uuid.UUID
	EventName      EventName
	Kind           *string
	Source         string
	SchemaVersion  int
	OccurredAt     time.Time
	SelectionState SelectionState
	ContextBizDate *time.Time
	KeywordID      *uuid.UUID
	ReceivedAt     time.Time
	BizDate        time.Time
	RequestID      string
}

type ClickMetric struct {
	EventName       EventName
	Kind            string
	ClickCount      int64
	UniqueUserCount int64
}
