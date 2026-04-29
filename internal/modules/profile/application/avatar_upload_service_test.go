package application

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildAvatarObjectKeyUsesStableImageExtensions(t *testing.T) {
	userID := uuid.MustParse("91111111-1111-1111-1111-111111111111")
	sessionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)

	got := buildAvatarObjectKey("uploads", userID, sessionID, "image/jpeg", now)
	wantSuffix := "/avatars/" + userID.String() + "/" + sessionID.String() + "/image.jpg"
	if !strings.HasSuffix(got, wantSuffix) {
		t.Fatalf("buildAvatarObjectKey() = %q, want suffix %q", got, wantSuffix)
	}
}
