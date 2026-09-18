package application

import (
	"errors"
	"testing"
	"time"

	dofficial "cixing/internal/modules/official/domain"
	"cixing/internal/shared/common"
)

func TestBusinessDateBoundsUsesShanghaiMidnight(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 14, 15, 59, 59, 0, time.UTC)
	bizDate, resetsAt := businessDateBounds(now)

	wantBizDate := time.Date(2026, time.September, 14, 0, 0, 0, 0, time.UTC)
	if !bizDate.Equal(wantBizDate) {
		t.Fatalf("businessDateBounds() bizDate = %s, want %s", bizDate, wantBizDate)
	}
	wantReset := time.Date(2026, time.September, 15, 0, 0, 0, 0, common.BusinessLocation())
	if !resetsAt.Equal(wantReset) {
		t.Fatalf("businessDateBounds() resetsAt = %s, want %s", resetsAt, wantReset)
	}

	nextBizDate, _ := businessDateBounds(now.Add(time.Second))
	wantNext := time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC)
	if !nextBizDate.Equal(wantNext) {
		t.Fatalf("next businessDateBounds() bizDate = %s, want %s", nextBizDate, wantNext)
	}
}

func TestParsePromptKind(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"intuition", "structure", "concept"} {
		got, err := parsePromptKind(kind)
		if err != nil {
			t.Fatalf("parsePromptKind(%q) error = %v", kind, err)
		}
		if got != dofficial.PromptKind(kind) {
			t.Fatalf("parsePromptKind(%q) = %q", kind, got)
		}
	}
	if _, err := parsePromptKind("invalid"); !errors.Is(err, ErrInvalidPromptKind) {
		t.Fatalf("parsePromptKind(invalid) error = %v, want ErrInvalidPromptKind", err)
	}
}
