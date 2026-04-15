package application

import (
	"testing"
	"time"

	"github.com/google/uuid"

	dofficial "cixing/internal/modules/official/domain"
	"cixing/internal/shared/common"
)

func TestRotatingKeywordForDateUsesDisplayOrderFromFixedAnchor(t *testing.T) {
	t.Parallel()

	first := dofficial.OfficialKeyword{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Text: "A", IsActive: true, DisplayOrder: 1}
	second := dofficial.OfficialKeyword{ID: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Text: "B", IsActive: true, DisplayOrder: 2}
	third := dofficial.OfficialKeyword{ID: uuid.MustParse("33333333-3333-3333-3333-333333333333"), Text: "C", IsActive: true, DisplayOrder: 3}

	cases := []struct {
		name    string
		bizDate time.Time
		wantID  uuid.UUID
	}{
		{name: "anchor day", bizDate: time.Date(2026, time.March, 20, 12, 0, 0, 0, common.BusinessLocation()), wantID: first.ID},
		{name: "next day", bizDate: time.Date(2026, time.March, 21, 9, 0, 0, 0, common.BusinessLocation()), wantID: second.ID},
		{name: "third day", bizDate: time.Date(2026, time.March, 22, 9, 0, 0, 0, common.BusinessLocation()), wantID: third.ID},
		{name: "wrap around", bizDate: time.Date(2026, time.March, 23, 9, 0, 0, 0, common.BusinessLocation()), wantID: first.ID},
		{name: "day before anchor", bizDate: time.Date(2026, time.March, 19, 9, 0, 0, 0, common.BusinessLocation()), wantID: third.ID},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := rotatingKeywordForDate([]dofficial.OfficialKeyword{third, first, second}, tc.bizDate)
			if got.ID != tc.wantID {
				t.Fatalf("rotatingKeywordForDate() keyword_id = %s, want %s", got.ID, tc.wantID)
			}
		})
	}
}
