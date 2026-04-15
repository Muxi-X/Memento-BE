package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	customapp "cixing/internal/modules/customkeywords/application"
	customdb "cixing/internal/modules/customkeywords/infra/db/gen"
	customrepo "cixing/internal/modules/customkeywords/infra/db/repo"
	publishingapp "cixing/internal/modules/publishing/application"
	dpub "cixing/internal/modules/publishing/domain"
	readmodelapp "cixing/internal/modules/readmodel/application"
	readmodeldb "cixing/internal/modules/readmodel/infra/db/gen"
	readmodelrepo "cixing/internal/modules/readmodel/infra/db/repo"
	platformoss "cixing/internal/platform/oss"
)

func TestCustomKeywordFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	storage := newMemoryObjectStorage()
	now := time.Now().UTC().Add(-2 * time.Second)

	userID := uuid.MustParse("71111111-1111-1111-1111-111111111111")
	seedUser(t, ctx, pool, userID, "custom-user@example.com", "Custom User")

	resolver := platformoss.NewURLResolver(platformoss.URLResolverConfig{
		PublicBaseURL: "https://cdn.test.local",
	})
	customKeywordRepo := customrepo.NewRepository(customdb.New(pool))
	customKeywordSvc := customapp.NewService(customKeywordRepo, resolver)
	publishingSvc := publishingapp.NewService(pool, nil, func() time.Time { return now }).WithCustomKeywords(customKeywordRepo)
	uploadSessionSvc, err := publishingapp.NewUploadSessionService(pool, publishingSvc, storage, publishingapp.UploadSessionServiceConfig{
		Bucket:           "test-bucket",
		UploadPrefix:     "it-uploads",
		PutPresignExpire: 15 * time.Minute,
		Now:              func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewUploadSessionService() error = %v", err)
	}
	readSvc := readmodelapp.NewService(
		readmodelrepo.NewRepository(readmodeldb.New(pool)),
		resolver,
		nil,
		func() time.Time { return now },
	).WithCustomKeywords(customKeywordSvc)

	createdKeyword, err := customKeywordSvc.Create(ctx, userID, "春天", intPtr(5))
	if err != nil {
		t.Fatalf("Create(custom keyword) error = %v", err)
	}
	if createdKeyword.CoverImage != nil {
		t.Fatalf("Create(custom keyword) cover image = %+v, want nil", createdKeyword.CoverImage)
	}

	createdSession, err := uploadSessionSvc.Create(ctx, publishingapp.CreateUploadSessionInput{
		OwnerUserID:     userID,
		ContextType:     dpub.PublishContextCustomKeyword,
		CustomKeywordID: uuidPtr(createdKeyword.ID),
	})
	if err != nil {
		t.Fatalf("Create(custom upload session) error = %v", err)
	}
	if createdSession.SessionID == uuid.Nil {
		t.Fatalf("Create(custom upload session) session id = nil, want non-nil")
	}
	if createdSession.Status != "created" {
		t.Fatalf("Create(custom upload session) status = %q, want created", createdSession.Status)
	}

	firstImage := buildJPEG(t, 1200, 1600)
	secondImage := buildJPEG(t, 1400, 1400)
	presigned, err := uploadSessionSvc.PresignBatch(ctx, publishingapp.PresignBatchInput{
		OwnerUserID: userID,
		SessionID:   createdSession.SessionID,
		Items: []publishingapp.PresignBatchItemInput{
			{
				ClientImageID:      "img-1",
				ImageContentType:   "image/jpeg",
				ImageContentLength: int64(len(firstImage)),
			},
			{
				ClientImageID:      "img-2",
				ImageContentType:   "image/jpeg",
				ImageContentLength: int64(len(secondImage)),
			},
		},
	})
	if err != nil {
		t.Fatalf("PresignBatch(custom) error = %v", err)
	}
	if len(presigned.Items) != 2 {
		t.Fatalf("PresignBatch(custom) items = %d, want 2", len(presigned.Items))
	}

	if err := storage.Put(ctx, "test-bucket", presigned.Items[0].ImageUpload.ObjectKey, firstImage, "image/jpeg"); err != nil {
		t.Fatalf("Put(first custom image) error = %v", err)
	}
	if err := storage.Put(ctx, "test-bucket", presigned.Items[1].ImageUpload.ObjectKey, secondImage, "image/jpeg"); err != nil {
		t.Fatalf("Put(second custom image) error = %v", err)
	}
	firstETag := mustHeadObjectETag(t, ctx, storage, "test-bucket", presigned.Items[0].ImageUpload.ObjectKey)
	secondETag := mustHeadObjectETag(t, ctx, storage, "test-bucket", presigned.Items[1].ImageUpload.ObjectKey)

	if _, err := uploadSessionSvc.CompleteBatch(ctx, publishingapp.CompleteBatchInput{
		OwnerUserID: userID,
		SessionID:   createdSession.SessionID,
		Items: []publishingapp.CompleteBatchItemInput{
			{
				ItemID:       presigned.Items[0].ItemID,
				ImageEtag:    firstETag,
				ImageWidth:   1200,
				ImageHeight:  1600,
				DisplayOrder: 1,
				IsCover:      true,
				Title:        strPtr("第一张"),
				Note:         strPtr("第一张文案"),
			},
			{
				ItemID:       presigned.Items[1].ItemID,
				ImageEtag:    secondETag,
				ImageWidth:   1400,
				ImageHeight:  1400,
				DisplayOrder: 2,
				Title:        strPtr("第二张"),
				Note:         strPtr("第二张文案"),
			},
		},
	}); err != nil {
		t.Fatalf("CompleteBatch(custom) error = %v", err)
	}

	committed, err := uploadSessionSvc.Commit(ctx, publishingapp.CommitUploadSessionInput{
		OwnerUserID: userID,
		SessionID:   createdSession.SessionID,
	})
	if err != nil {
		t.Fatalf("Commit(custom) error = %v", err)
	}
	if committed.UploadVisibility != dpub.WorkUploadVisibilityVisible {
		t.Fatalf("Commit(custom) upload visibility = %q, want visible", committed.UploadVisibility)
	}
	assertJobCount(t, ctx, pool, 0)

	meHome, err := readSvc.GetMeHome(ctx, userID)
	if err != nil {
		t.Fatalf("GetMeHome(custom) error = %v", err)
	}
	if meHome.CustomImageCount != 2 {
		t.Fatalf("GetMeHome(custom) custom image count = %d, want 2", meHome.CustomImageCount)
	}
	if len(meHome.CustomKeywords) != 1 {
		t.Fatalf("GetMeHome(custom) custom keywords = %d, want 1", len(meHome.CustomKeywords))
	}
	if meHome.CustomKeywords[0].CoverImage == nil || !strings.Contains(meHome.CustomKeywords[0].CoverImage.SquareSmall.URL, "square_small") {
		t.Fatalf("GetMeHome(custom) cover image = %+v, want square_small url", meHome.CustomKeywords[0].CoverImage)
	}

	gallery, err := customKeywordSvc.ListImages(ctx, userID, createdKeyword.ID, 50)
	if err != nil {
		t.Fatalf("ListImages(custom) error = %v", err)
	}
	if gallery.CoverSource != "auto_latest" {
		t.Fatalf("ListImages(custom) cover source = %q, want auto_latest", gallery.CoverSource)
	}
	if gallery.CoverImage == nil || !strings.Contains(gallery.CoverImage.DetailLarge.URL, presigned.Items[0].ImageUpload.ObjectKey) {
		t.Fatalf("ListImages(custom) cover image = %+v, want first image detail_large", gallery.CoverImage)
	}
	if len(gallery.Items) != 2 {
		t.Fatalf("ListImages(custom) items = %d, want 2", len(gallery.Items))
	}
	if !strings.Contains(gallery.Items[0].Image.SquareMedium.URL, "square_medium") {
		t.Fatalf("ListImages(custom) first item square_medium url = %q", gallery.Items[0].Image.SquareMedium.URL)
	}

	detail, err := customKeywordSvc.GetImage(ctx, userID, gallery.Items[1].ID)
	if err != nil {
		t.Fatalf("GetImage(custom) error = %v", err)
	}
	if detail.Title == nil || *detail.Title != "第二张" {
		t.Fatalf("GetImage(custom) title = %v, want 第二张", detail.Title)
	}
	if !strings.Contains(detail.Image.DetailLarge.URL, presigned.Items[1].ImageUpload.ObjectKey) {
		t.Fatalf("GetImage(custom) detail_large url = %q, want second object key", detail.Image.DetailLarge.URL)
	}

	setCover, err := customKeywordSvc.SetCover(ctx, userID, createdKeyword.ID, gallery.Items[1].ID)
	if err != nil {
		t.Fatalf("SetCover(custom) error = %v", err)
	}
	if setCover.CoverSource != "manual" {
		t.Fatalf("SetCover(custom) cover source = %q, want manual", setCover.CoverSource)
	}
	if setCover.CoverImage == nil || !strings.Contains(setCover.CoverImage.DetailLarge.URL, presigned.Items[1].ImageUpload.ObjectKey) {
		t.Fatalf("SetCover(custom) cover image = %+v, want second image detail_large", setCover.CoverImage)
	}

	clearedCover, err := customKeywordSvc.ClearCover(ctx, userID, createdKeyword.ID)
	if err != nil {
		t.Fatalf("ClearCover(custom) error = %v", err)
	}
	if clearedCover.CoverSource != "auto_latest" {
		t.Fatalf("ClearCover(custom) cover source = %q, want auto_latest", clearedCover.CoverSource)
	}
	if clearedCover.CoverImage == nil || !strings.Contains(clearedCover.CoverImage.DetailLarge.URL, presigned.Items[0].ImageUpload.ObjectKey) {
		t.Fatalf("ClearCover(custom) cover image = %+v, want first image detail_large", clearedCover.CoverImage)
	}
}
