package common

import (
	"testing"
	"time"
)

func TestNormalizeBizDateUsesShanghaiCalendarDay(t *testing.T) {
	t.Parallel()

	in := time.Date(2026, time.March, 19, 18, 30, 0, 0, time.UTC) // 2026-03-20 02:30 in Asia/Shanghai
	got := NormalizeBizDate(in)
	want := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("NormalizeBizDate() = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}
