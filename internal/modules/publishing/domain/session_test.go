package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"cixing/internal/shared/common"
)

func TestAggregateBeginOfficialCommit(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	keywordID := uuid.New()
	coverID := uuid.New()
	otherID := uuid.New()
	bizDate := time.Date(2026, 3, 18, 0, 0, 0, 0, time.UTC)

	agg := Aggregate{
		Session: NewOfficialSession(uuid.New(), keywordID, bizDate, now, now.Add(48*time.Hour)),
		Items: []PublishSessionItem{
			{ID: uuid.New(), ImageAssetID: otherID, Status: ItemStatusUploaded, DisplayOrder: int32Ptr(2)},
			{ID: uuid.New(), ImageAssetID: coverID, Status: ItemStatusUploaded, DisplayOrder: int32Ptr(1), IsCover: true},
		},
	}
	ordered, cover, err := agg.BeginOfficialCommit(now, keywordID)
	if err != nil {
		t.Fatalf("BeginOfficialCommit() error = %v", err)
	}
	if agg.Session.Status != SessionStatusCreated {
		t.Fatalf("session status = %s, want %s", agg.Session.Status, SessionStatusCreated)
	}
	if len(ordered) != 2 {
		t.Fatalf("ordered len = %d, want 2", len(ordered))
	}
	if ordered[0].ImageAssetID != coverID {
		t.Fatalf("ordered[0].ImageAssetID = %s, want %s", ordered[0].ImageAssetID, coverID)
	}
	if cover.ImageAssetID != coverID {
		t.Fatalf("cover.ImageAssetID = %s, want %s", cover.ImageAssetID, coverID)
	}
}

func TestAggregateBeginOfficialCommitRejectsMissingCover(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	keywordID := uuid.New()
	agg := Aggregate{
		Session: NewOfficialSession(uuid.New(), keywordID, now, now, now.Add(48*time.Hour)),
		Items: []PublishSessionItem{
			{ID: uuid.New(), Status: ItemStatusUploaded, DisplayOrder: int32Ptr(1)},
		},
	}
	if _, _, err := agg.BeginOfficialCommit(now, keywordID); !errors.Is(err, common.ErrConflict) {
		t.Fatalf("BeginOfficialCommit() error = %v, want conflict", err)
	}
}
