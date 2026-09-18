package application

import (
	"testing"
	"time"

	"github.com/google/uuid"

	danalytics "cixing/internal/modules/analytics/domain"
)

func TestValidateBatchEventCombinations(t *testing.T) {
	t.Parallel()

	kind := "structure"
	validMain := danalytics.Event{
		EventID:        uuid.New(),
		EventName:      danalytics.EventNamePromptEntryClick,
		Source:         "today",
		SchemaVersion:  1,
		OccurredAt:     time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC),
		SelectionState: danalytics.SelectionStateUnknown,
	}
	validKind := validMain
	validKind.EventName = danalytics.EventNamePromptKindEntryClick
	validKind.Kind = &kind
	validKind.SelectionState = danalytics.SelectionStateUnselected

	tests := []struct {
		name    string
		events  []danalytics.Event
		wantErr bool
		field   string
	}{
		{name: "valid main", events: []danalytics.Event{validMain}},
		{name: "valid kind", events: []danalytics.Event{validKind}},
		{name: "main kind must be null", events: []danalytics.Event{func() danalytics.Event { e := validMain; e.Kind = &kind; return e }()}, wantErr: true, field: "kind"},
		{name: "kind entry requires kind", events: []danalytics.Event{func() danalytics.Event { e := validKind; e.Kind = nil; return e }()}, wantErr: true, field: "kind"},
		{name: "kind entry requires unselected", events: []danalytics.Event{func() danalytics.Event {
			e := validKind
			e.SelectionState = danalytics.SelectionStateSelected
			return e
		}()}, wantErr: true, field: "selection_state"},
		{name: "batch cannot be empty", events: nil, wantErr: true, field: "events"},
		{name: "batch cannot exceed 20", events: make([]danalytics.Event, 21), wantErr: true, field: "events"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateBatch(tt.events)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("validateBatch() error = %v", err)
				}
				return
			}
			validationErr, ok := err.(*ValidationError)
			if !ok || len(validationErr.Fields) == 0 {
				t.Fatalf("validateBatch() error = %T %v, want ValidationError", err, err)
			}
			if validationErr.Fields[0].Field != tt.field {
				t.Fatalf("validation field = %q, want %q", validationErr.Fields[0].Field, tt.field)
			}
		})
	}
}

func TestDeduplicateEventsKeepsFirstPayload(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	first := danalytics.Event{EventID: id, Source: "today", SchemaVersion: 1}
	second := danalytics.Event{EventID: id, Source: "later", SchemaVersion: 2}

	got := deduplicateEvents([]danalytics.Event{first, second})
	if len(got) != 1 {
		t.Fatalf("deduplicateEvents() len = %d, want 1", len(got))
	}
	if got[0].Source != first.Source || got[0].SchemaVersion != first.SchemaVersion {
		t.Fatalf("deduplicateEvents() kept %+v, want first payload %+v", got[0], first)
	}
}
