package application

import (
	"testing"
	"time"
)

func TestServiceShouldLazyAssignOfficialDate(t *testing.T) {
	now := time.Date(2026, time.April, 15, 10, 30, 0, 0, time.UTC)
	svc := &Service{
		now: func() time.Time { return now },
	}

	tests := []struct {
		name string
		date time.Time
		want bool
	}{
		{
			name: "today",
			date: now,
			want: true,
		},
		{
			name: "yesterday",
			date: now.AddDate(0, 0, -1),
			want: true,
		},
		{
			name: "tomorrow",
			date: now.AddDate(0, 0, 1),
			want: false,
		},
		{
			name: "older history",
			date: now.AddDate(0, 0, -2),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.shouldLazyAssignOfficialDate(tt.date); got != tt.want {
				t.Fatalf("shouldLazyAssignOfficialDate(%s) = %v, want %v", tt.date.Format("2006-01-02"), got, tt.want)
			}
		})
	}
}
