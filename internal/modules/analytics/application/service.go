package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	danalytics "cixing/internal/modules/analytics/domain"
	analyticsdb "cixing/internal/modules/analytics/infra/db/gen"
	analyticsrepo "cixing/internal/modules/analytics/infra/db/repo"
	"cixing/internal/platform/postgres"
	"cixing/internal/shared/common"
)

const maxBatchEvents = 20

var ErrUnauthenticatedUser = errors.New("unauthenticated user")

type EventFieldError struct {
	EventIndex int
	Field      string
	Rule       string
	Reason     string
}

type ValidationError struct {
	Fields []EventFieldError
}

func (e *ValidationError) Error() string {
	if e == nil || len(e.Fields) == 0 {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed for %s", e.Fields[0].Field)
}

type Service struct {
	db   *pgxpool.Pool
	repo danalytics.Repository
	now  func() time.Time
}

func NewService(db *pgxpool.Pool, repo danalytics.Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{db: db, repo: repo, now: now}
}

func (s *Service) RecordBatch(ctx context.Context, userID uuid.UUID, requestID string, events []danalytics.Event) ([]uuid.UUID, error) {
	if userID == uuid.Nil {
		return nil, ErrUnauthenticatedUser
	}
	if err := validateBatch(events); err != nil {
		return nil, err
	}
	if s.db == nil {
		return nil, errors.New("analytics service is not configured")
	}

	uniqueEvents := deduplicateEvents(events)
	acknowledged := make([]uuid.UUID, 0, len(uniqueEvents))
	for _, event := range uniqueEvents {
		acknowledged = append(acknowledged, event.EventID)
	}

	receivedAt := s.now()
	bizDate := common.NormalizeBizDate(receivedAt)
	err := postgres.WithTx(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		repo := analyticsrepo.NewRepository(analyticsdb.New(tx))
		for _, event := range uniqueEvents {
			if err := repo.InsertEvent(ctx, danalytics.InsertEventParams{
				UserID:         userID,
				EventID:        event.EventID,
				EventName:      event.EventName,
				Kind:           event.Kind,
				Source:         event.Source,
				SchemaVersion:  event.SchemaVersion,
				OccurredAt:     event.OccurredAt,
				SelectionState: event.SelectionState,
				ContextBizDate: event.ContextBizDate,
				KeywordID:      event.KeywordID,
				ReceivedAt:     receivedAt,
				BizDate:        bizDate,
				RequestID:      requestID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return acknowledged, nil
}

func (s *Service) ClickMetrics(ctx context.Context, bizDate time.Time) ([]danalytics.ClickMetric, error) {
	return s.repo.GetClickMetrics(ctx, bizDate)
}

func (s *Service) DailyPromptSuccessCount(ctx context.Context, bizDate time.Time) (int64, error) {
	return s.repo.GetDailyPromptSuccessCount(ctx, bizDate)
}

func validateBatch(events []danalytics.Event) error {
	fields := make([]EventFieldError, 0)
	if len(events) < 1 || len(events) > maxBatchEvents {
		fields = append(fields, EventFieldError{
			EventIndex: -1,
			Field:      "events",
			Rule:       "size",
			Reason:     "must contain between 1 and 20 items",
		})
		return &ValidationError{Fields: fields}
	}

	for i, event := range events {
		fields = append(fields, validateEvent(i, event)...)
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

func validateEvent(index int, event danalytics.Event) []EventFieldError {
	fields := make([]EventFieldError, 0)
	add := func(field, rule, reason string) {
		fields = append(fields, EventFieldError{EventIndex: index, Field: field, Rule: rule, Reason: reason})
	}

	if event.EventID == uuid.Nil {
		add("event_id", "uuid", "must be a valid UUID")
	}
	if event.Source != "today" {
		add("source", "oneof", "must be today")
	}
	if event.SchemaVersion != 1 {
		add("schema_version", "oneof", "must be 1")
	}
	if event.ContextBizDate != nil && !isRealCalendarDate(*event.ContextBizDate) {
		add("context_biz_date", "date", "must be a real calendar date")
	}

	switch event.EventName {
	case danalytics.EventNamePromptEntryClick:
		if event.Kind != nil {
			add("kind", "null", "must be null for prompt_entry_click")
		}
		if !validSelectionState(event.SelectionState) {
			add("selection_state", "oneof", "must be one of unknown, unselected, selected")
		}
	case danalytics.EventNamePromptKindEntryClick:
		if event.Kind == nil || !validPromptKind(*event.Kind) {
			add("kind", "oneof", "must be one of intuition, structure, concept")
		}
		if event.SelectionState != danalytics.SelectionStateUnselected {
			add("selection_state", "oneof", "must be unselected for prompt_kind_entry_click")
		}
	default:
		add("event_name", "oneof", "must be one of prompt_entry_click, prompt_kind_entry_click")
	}
	return fields
}

func validSelectionState(state danalytics.SelectionState) bool {
	switch state {
	case danalytics.SelectionStateUnknown, danalytics.SelectionStateUnselected, danalytics.SelectionStateSelected:
		return true
	default:
		return false
	}
}

func validPromptKind(kind string) bool {
	switch kind {
	case "intuition", "structure", "concept":
		return true
	default:
		return false
	}
}

func isRealCalendarDate(value time.Time) bool {
	if value.IsZero() {
		return false
	}
	year, month, day := value.Date()
	rebuilt := time.Date(year, month, day, 0, 0, 0, 0, value.Location())
	return rebuilt.Year() == year && rebuilt.Month() == month && rebuilt.Day() == day
}

func deduplicateEvents(events []danalytics.Event) []danalytics.Event {
	seen := make(map[uuid.UUID]struct{}, len(events))
	out := make([]danalytics.Event, 0, len(events))
	for _, event := range events {
		if _, ok := seen[event.EventID]; ok {
			continue
		}
		seen[event.EventID] = struct{}{}
		out = append(out, event)
	}
	return out
}
