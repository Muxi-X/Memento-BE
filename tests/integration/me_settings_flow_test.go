package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	profileapp "cixing/internal/modules/profile/application"
	profiledb "cixing/internal/modules/profile/infra/db/gen"
	profilerepo "cixing/internal/modules/profile/infra/db/repo"
	socialapp "cixing/internal/modules/social/application"
	platformoss "cixing/internal/platform/oss"
)

func TestMeSettingsFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	storage := newMemoryObjectStorage()
	now := time.Now().UTC()

	userID := uuid.MustParse("81111111-1111-1111-1111-111111111111")
	authorID := uuid.MustParse("82222222-2222-2222-2222-222222222222")
	keywordID := uuid.MustParse("83333333-3333-3333-3333-333333333333")
	bizDate := dateOnly(now)

	seedUser(t, ctx, pool, userID, "settings-user@example.com", "Old Nick")
	seedUser(t, ctx, pool, authorID, "settings-author@example.com", "Author")
	seedOfficialKeywordAsDaily(t, ctx, pool, keywordID, "keyword", bizDate)

	resolver := platformoss.NewURLResolver(platformoss.URLResolverConfig{
		PublicBaseURL: "https://cdn.test.local",
	})
	profileSvc := profileapp.NewService(profilerepo.NewRepository(profiledb.New(pool)), resolver)
	reactionSvc := socialapp.NewReactionService(pool)

	settings, err := profileSvc.GetSettings(ctx, userID)
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	if settings.Profile.Nickname != "Old Nick" {
		t.Fatalf("GetSettings() nickname = %q, want Old Nick", settings.Profile.Nickname)
	}
	if settings.Profile.Email == nil || *settings.Profile.Email != "settings-user@example.com" {
		t.Fatalf("GetSettings() email = %v, want settings-user@example.com", settings.Profile.Email)
	}
	if !settings.Notifications.ReactionEnabled {
		t.Fatalf("GetSettings() reaction enabled = false, want true")
	}

	updatedNotifications, err := profileSvc.UpdateReactionNotifications(ctx, userID, boolPtr(false))
	if err != nil {
		t.Fatalf("UpdateReactionNotifications(false) error = %v", err)
	}
	if updatedNotifications.Notifications.ReactionEnabled {
		t.Fatalf("UpdateReactionNotifications(false) reaction enabled = true, want false")
	}

	updatedProfile, err := profileSvc.UpdateNickname(ctx, userID, "Renamed Actor")
	if err != nil {
		t.Fatalf("UpdateNickname() error = %v", err)
	}
	if updatedProfile.Nickname != "Renamed Actor" {
		t.Fatalf("UpdateNickname() nickname = %q, want Renamed Actor", updatedProfile.Nickname)
	}

	uploadID := createVisibleOfficialUploadForTest(t, ctx, pool, storage, authorID, keywordID, bizDate, now)
	if _, err := profileSvc.UpdateReactionNotifications(ctx, authorID, boolPtr(true)); err != nil {
		t.Fatalf("UpdateReactionNotifications(author true) error = %v", err)
	}
	if err := reactionSvc.React(ctx, userID, uploadID, "inspired"); err != nil {
		t.Fatalf("React(after nickname update) error = %v", err)
	}

	var actorNicknameSnapshot string
	if err := pool.QueryRow(ctx, `
SELECT actor_nickname_snapshot
FROM notifications
WHERE recipient_user_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1
`, authorID).Scan(&actorNicknameSnapshot); err != nil {
		t.Fatalf("query notification snapshot nickname: %v", err)
	}
	if actorNicknameSnapshot != "Renamed Actor" {
		t.Fatalf("notification actor_nickname_snapshot = %q, want Renamed Actor", actorNicknameSnapshot)
	}
}
