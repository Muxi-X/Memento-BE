package media

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAssetLifecycle(t *testing.T) {
	width := int32(100)
	height := int32(80)

	asset := Asset{
		ID:                uuid.New(),
		OriginalObjectKey: "media/original.jpg",
		Status:            AssetStatusPendingUpload,
	}

	if err := asset.MarkUploaded(&width, &height, nil); err != nil {
		t.Fatalf("MarkUploaded() error = %v", err)
	}
	if asset.Status != AssetStatusUploaded {
		t.Fatalf("expected uploaded, got %s", asset.Status)
	}

	if err := asset.StartProcessing(); err != nil {
		t.Fatalf("StartProcessing() error = %v", err)
	}
	if err := asset.MarkReady(); err != nil {
		t.Fatalf("MarkReady() error = %v", err)
	}
	if asset.Status != AssetStatusReady {
		t.Fatalf("expected ready, got %s", asset.Status)
	}

	now := time.Now().UTC()
	if err := asset.SoftDelete(now); err != nil {
		t.Fatalf("SoftDelete() error = %v", err)
	}
	if asset.Status != AssetStatusDeleted {
		t.Fatalf("expected deleted, got %s", asset.Status)
	}
	if asset.DeletedAt == nil {
		t.Fatal("expected deleted_at to be set")
	}
}

func TestAssetRejectsInvalidTransition(t *testing.T) {
	asset := Asset{Status: AssetStatusReady}
	if err := asset.StartProcessing(); err == nil {
		t.Fatal("expected invalid transition error")
	}
}
