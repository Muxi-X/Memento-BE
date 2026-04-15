package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	officialapp "cixing/internal/modules/official/application"
	officialdb "cixing/internal/modules/official/infra/db/gen"
	officialrepo "cixing/internal/modules/official/infra/db/repo"
	profileapp "cixing/internal/modules/profile/application"
	profiledb "cixing/internal/modules/profile/infra/db/gen"
	profilerepo "cixing/internal/modules/profile/infra/db/repo"
	readmodelapp "cixing/internal/modules/readmodel/application"
	readmodeldb "cixing/internal/modules/readmodel/infra/db/gen"
	readmodelrepo "cixing/internal/modules/readmodel/infra/db/repo"
	socialapp "cixing/internal/modules/social/application"
	platformoss "cixing/internal/platform/oss"
)

func TestReactionNotificationFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	storage := newMemoryObjectStorage()
	now := time.Now().UTC()

	authorID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	actorID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	keywordID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	bizDate := dateOnly(now)

	seedUser(t, ctx, pool, authorID, "author@example.com", "Author")
	seedUser(t, ctx, pool, actorID, "actor@example.com", "Actor")
	seedOfficialKeyword(t, ctx, pool, keywordID, "keyword", bizDate)
	officialCatalog := officialapp.NewCatalogService(officialrepo.NewRepository(officialdb.New(pool)), func() time.Time { return now })

	resolver := platformoss.NewURLResolver(platformoss.URLResolverConfig{
		PublicBaseURL: "https://cdn.test.local",
	})

	uploadID := createVisibleOfficialUploadForTest(t, ctx, pool, storage, authorID, keywordID, bizDate, now)
	secondUploadID := createVisibleOfficialUploadForTest(t, ctx, pool, storage, authorID, keywordID, bizDate, now.Add(2*time.Second))

	reactionSvc := socialapp.NewReactionService(pool)
	notificationSvc := socialapp.NewNotificationService(pool, resolver, func() time.Time { return now })
	profileSvc := profileapp.NewService(profilerepo.NewRepository(profiledb.New(pool)), resolver)
	readSvc := readmodelapp.NewService(
		readmodelrepo.NewRepository(readmodeldb.New(pool)),
		resolver,
		officialCatalog,
		func() time.Time { return now },
	)

	if err := reactionSvc.React(ctx, actorID, uploadID, "inspired"); err != nil {
		t.Fatalf("React(inspired) error = %v", err)
	}
	if err := reactionSvc.React(ctx, actorID, uploadID, "resonated"); err != nil {
		t.Fatalf("React(resonated) error = %v", err)
	}
	if err := reactionSvc.React(ctx, actorID, uploadID, "inspired"); err != nil {
		t.Fatalf("React(duplicate inspired) error = %v, want nil", err)
	}

	detail, err := readSvc.GetOfficialUpload(ctx, uploadID, true, &actorID)
	if err != nil {
		t.Fatalf("GetOfficialUpload() error = %v", err)
	}
	if detail.ReactionCounts == nil {
		t.Fatalf("GetOfficialUpload() reaction counts = nil")
	}
	if detail.ReactionCounts.Inspired != 1 || detail.ReactionCounts.Resonated != 1 {
		t.Fatalf("GetOfficialUpload() counts = %+v, want inspired=1 resonated=1", *detail.ReactionCounts)
	}
	if !contains(detail.MyReactions, "inspired") || !contains(detail.MyReactions, "resonated") {
		t.Fatalf("GetOfficialUpload() my reactions = %v, want inspired and resonated", detail.MyReactions)
	}

	notifications, err := notificationSvc.List(ctx, authorID)
	if err != nil {
		t.Fatalf("List(author notifications) error = %v", err)
	}
	if len(notifications.Items) != 2 {
		t.Fatalf("List(author notifications) items = %d, want 2", len(notifications.Items))
	}
	if notifications.Items[0].CoverImage == nil {
		t.Fatalf("List(author notifications) cover image = nil")
	}

	meHome, err := readSvc.GetMeHome(ctx, authorID)
	if err != nil {
		t.Fatalf("GetMeHome(author after react) error = %v", err)
	}
	if meHome.UnreadNotificationCount != 2 {
		t.Fatalf("GetMeHome(author after react) unread = %d, want 2", meHome.UnreadNotificationCount)
	}

	if err := reactionSvc.Unreact(ctx, actorID, uploadID, "inspired"); err != nil {
		t.Fatalf("Unreact(inspired) error = %v", err)
	}
	if err := reactionSvc.Unreact(ctx, actorID, uploadID, "inspired"); err != nil {
		t.Fatalf("Unreact(duplicate inspired) error = %v, want nil", err)
	}

	detailAfterUnreact, err := readSvc.GetOfficialUpload(ctx, uploadID, true, &actorID)
	if err != nil {
		t.Fatalf("GetOfficialUpload(after unreact) error = %v", err)
	}
	if detailAfterUnreact.ReactionCounts == nil {
		t.Fatalf("GetOfficialUpload(after unreact) reaction counts = nil")
	}
	if detailAfterUnreact.ReactionCounts.Inspired != 0 || detailAfterUnreact.ReactionCounts.Resonated != 1 {
		t.Fatalf("GetOfficialUpload(after unreact) counts = %+v, want inspired=0 resonated=1", *detailAfterUnreact.ReactionCounts)
	}
	if contains(detailAfterUnreact.MyReactions, "inspired") {
		t.Fatalf("GetOfficialUpload(after unreact) my reactions = %v, should not contain inspired", detailAfterUnreact.MyReactions)
	}
	if !contains(detailAfterUnreact.MyReactions, "resonated") {
		t.Fatalf("GetOfficialUpload(after unreact) my reactions = %v, should contain resonated", detailAfterUnreact.MyReactions)
	}

	if err := reactionSvc.React(ctx, authorID, uploadID, "inspired"); err != nil {
		t.Fatalf("React(self inspired) error = %v", err)
	}
	notificationsAfterSelfReact, err := notificationSvc.List(ctx, authorID)
	if err != nil {
		t.Fatalf("List(author notifications after self react) error = %v", err)
	}
	if len(notificationsAfterSelfReact.Items) != 2 {
		t.Fatalf("List(author notifications after self react) items = %d, want 2", len(notificationsAfterSelfReact.Items))
	}

	if _, err := profileSvc.UpdateReactionNotifications(ctx, authorID, boolPtr(false)); err != nil {
		t.Fatalf("UpdateReactionNotifications(false) error = %v", err)
	}
	if err := reactionSvc.React(ctx, actorID, secondUploadID, "inspired"); err != nil {
		t.Fatalf("React(second upload inspired) error = %v", err)
	}
	notificationsAfterDisabled, err := notificationSvc.List(ctx, authorID)
	if err != nil {
		t.Fatalf("List(author notifications after disabling setting) error = %v", err)
	}
	if len(notificationsAfterDisabled.Items) != 2 {
		t.Fatalf("List(author notifications after disabling setting) items = %d, want 2", len(notificationsAfterDisabled.Items))
	}

	if err := notificationSvc.MarkAllRead(ctx, authorID); err != nil {
		t.Fatalf("MarkAllRead(author notifications) error = %v", err)
	}
	notificationsAfterRead, err := notificationSvc.List(ctx, authorID)
	if err != nil {
		t.Fatalf("List(author notifications after mark all read) error = %v", err)
	}
	for _, item := range notificationsAfterRead.Items {
		if item.ReadAt == nil {
			t.Fatalf("notification %s read_at = nil, want non-nil", item.ID)
		}
	}
	meHomeAfterRead, err := readSvc.GetMeHome(ctx, authorID)
	if err != nil {
		t.Fatalf("GetMeHome(author after mark all read) error = %v", err)
	}
	if meHomeAfterRead.UnreadNotificationCount != 0 {
		t.Fatalf("GetMeHome(author after mark all read) unread = %d, want 0", meHomeAfterRead.UnreadNotificationCount)
	}
}
