package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	officialapp "cixing/internal/modules/official/application"
	officialdb "cixing/internal/modules/official/infra/db/gen"
	officialrepo "cixing/internal/modules/official/infra/db/repo"
	publishingapp "cixing/internal/modules/publishing/application"
	dpub "cixing/internal/modules/publishing/domain"
	readmodelapp "cixing/internal/modules/readmodel/application"
	readmodeldb "cixing/internal/modules/readmodel/infra/db/gen"
	readmodelrepo "cixing/internal/modules/readmodel/infra/db/repo"
	platformoss "cixing/internal/platform/oss"
	"cixing/internal/shared/common"
)

func TestOfficialUploadPublishFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	storage := newMemoryObjectStorage()
	now := time.Now().UTC().Add(-2 * time.Second)

	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	viewerUserID := uuid.MustParse("12111111-1111-1111-1111-111111111111")
	keywordID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	bizDate := dateOnly(now)

	seedUser(t, ctx, pool, userID, "it-user@example.com", "Integration User")
	seedUser(t, ctx, pool, viewerUserID, "review-viewer@example.com", "Review Viewer")
	seedOfficialKeyword(t, ctx, pool, keywordID, "晴朗", bizDate)
	officialCatalog := officialapp.NewCatalogService(officialrepo.NewRepository(officialdb.New(pool)), func() time.Time { return now })

	publishingSvc := publishingapp.NewService(pool, officialCatalog, func() time.Time { return now })
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
		platformoss.NewURLResolver(platformoss.URLResolverConfig{
			PublicBaseURL: "https://cdn.test.local",
		}),
		officialCatalog,
		func() time.Time { return now },
	)

	created, err := uploadSessionSvc.Create(ctx, publishingapp.CreateUploadSessionInput{
		OwnerUserID:       userID,
		ContextType:       "official_today",
		OfficialKeywordID: &keywordID,
		BizDate:           &bizDate,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Status != "created" {
		t.Fatalf("Create() status = %q, want created", created.Status)
	}

	imageBody := buildJPEG(t, 1200, 1600)
	presigned, err := uploadSessionSvc.PresignBatch(ctx, publishingapp.PresignBatchInput{
		OwnerUserID: userID,
		SessionID:   created.SessionID,
		Items: []publishingapp.PresignBatchItemInput{
			{
				ClientImageID:      "cover-1",
				ImageContentType:   "image/jpeg",
				ImageContentLength: int64(len(imageBody)),
			},
		},
	})
	if err != nil {
		t.Fatalf("PresignBatch() error = %v", err)
	}
	if len(presigned.Items) != 1 {
		t.Fatalf("PresignBatch() items = %d, want 1", len(presigned.Items))
	}

	target := presigned.Items[0].ImageUpload
	if err := storage.Put(ctx, "test-bucket", target.ObjectKey, imageBody, "image/jpeg"); err != nil {
		t.Fatalf("memory storage Put() error = %v", err)
	}
	imageETag := mustHeadObjectETag(t, ctx, storage, "test-bucket", target.ObjectKey)

	completed, err := uploadSessionSvc.CompleteBatch(ctx, publishingapp.CompleteBatchInput{
		OwnerUserID: userID,
		SessionID:   created.SessionID,
		Items: []publishingapp.CompleteBatchItemInput{
			{
				ItemID:       presigned.Items[0].ItemID,
				ImageEtag:    imageETag,
				ImageWidth:   1200,
				ImageHeight:  1600,
				DisplayOrder: 1,
				IsCover:      true,
				Title:        strPtr("一张图"),
				Note:         strPtr("第一张"),
			},
		},
	})
	if err != nil {
		t.Fatalf("CompleteBatch() error = %v", err)
	}
	if completed.Status != "created" {
		t.Fatalf("CompleteBatch() status = %q, want created", completed.Status)
	}

	beforeJobs, err := readSvc.ListOfficialDateUploads(ctx, bizDate, "latest", 20, nil, true, nil)
	if err != nil {
		t.Fatalf("ListOfficialDateUploads(before jobs) error = %v", err)
	}
	if len(beforeJobs.Items) != 0 {
		t.Fatalf("ListOfficialDateUploads(before jobs) items = %d, want 0", len(beforeJobs.Items))
	}

	committed, err := uploadSessionSvc.Commit(ctx, publishingapp.CommitUploadSessionInput{
		OwnerUserID: userID,
		SessionID:   created.SessionID,
	})
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if committed.Status != "committed" {
		t.Fatalf("Commit() status = %q, want committed", committed.Status)
	}
	if committed.UploadVisibility != dpub.WorkUploadVisibilityVisible {
		t.Fatalf("Commit() upload visibility = %q, want visible", committed.UploadVisibility)
	}
	assertJobCount(t, ctx, pool, 0)

	afterJobs, err := readSvc.ListOfficialDateUploads(ctx, bizDate, "latest", 20, nil, true, &userID)
	if err != nil {
		t.Fatalf("ListOfficialDateUploads(after jobs) error = %v", err)
	}
	if len(afterJobs.Items) != 1 {
		t.Fatalf("ListOfficialDateUploads(after jobs) items = %d, want 1", len(afterJobs.Items))
	}
	card := afterJobs.Items[0]
	if card.ID != committed.UploadID {
		t.Fatalf("visible upload id = %s, want %s", card.ID, committed.UploadID)
	}
	if card.CoverImage.Card4x3.URL == "" || !strings.Contains(card.CoverImage.Card4x3.URL, "card_4x3") {
		t.Fatalf("cover card url = %q, want card_4x3 url", card.CoverImage.Card4x3.URL)
	}

	home, err := readSvc.GetOfficialHome(ctx, &bizDate)
	if err != nil {
		t.Fatalf("GetOfficialHome() error = %v", err)
	}
	if home.Today.ParticipantUserCount != 1 {
		t.Fatalf("GetOfficialHome() participant_user_count = %d, want 1", home.Today.ParticipantUserCount)
	}

	detail, err := readSvc.GetOfficialUpload(ctx, committed.UploadID, true, &userID)
	if err != nil {
		t.Fatalf("GetOfficialUpload() error = %v", err)
	}
	if len(detail.Images) != 1 {
		t.Fatalf("GetOfficialUpload() images = %d, want 1", len(detail.Images))
	}
	if got := detail.Images[0].Image.DetailLarge.URL; !strings.Contains(got, target.ObjectKey) {
		t.Fatalf("detail_large url = %q, want object key %q", got, target.ObjectKey)
	}
	if detail.Images[0].Title == nil || *detail.Images[0].Title != "一张图" {
		t.Fatalf("detail title = %v, want 一张图", detail.Images[0].Title)
	}
	if detail.Images[0].Note == nil || *detail.Images[0].Note != "第一张" {
		t.Fatalf("detail note = %v, want 第一张", detail.Images[0].Note)
	}

	reviewDetail, err := readSvc.GetReviewUpload(ctx, viewerUserID, committed.UploadID)
	if err != nil {
		t.Fatalf("GetReviewUpload(non-owner) error = %v", err)
	}
	if len(reviewDetail.Images) != 1 {
		t.Fatalf("GetReviewUpload(non-owner) images = %d, want 1", len(reviewDetail.Images))
	}
	if got := reviewDetail.Images[0].Image.DetailLarge.URL; !strings.Contains(got, target.ObjectKey) {
		t.Fatalf("GetReviewUpload(non-owner) detail_large url = %q, want object key %q", got, target.ObjectKey)
	}
}

func TestOfficialUploadPublishFlow_CompleteBatchRejectsMismatchedETag(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	storage := newMemoryObjectStorage()
	now := time.Now().UTC().Add(-2 * time.Second)

	userID := uuid.MustParse("31111111-1111-1111-1111-111111111111")
	keywordID := uuid.MustParse("32222222-2222-2222-2222-222222222222")
	bizDate := dateOnly(now)

	seedUser(t, ctx, pool, userID, "etag-user@example.com", "ETag User")
	seedOfficialKeyword(t, ctx, pool, keywordID, "晴朗", bizDate)
	officialCatalog := officialapp.NewCatalogService(officialrepo.NewRepository(officialdb.New(pool)), func() time.Time { return now })

	publishingSvc := publishingapp.NewService(pool, officialCatalog, func() time.Time { return now })
	uploadSessionSvc, err := publishingapp.NewUploadSessionService(pool, publishingSvc, storage, publishingapp.UploadSessionServiceConfig{
		Bucket:           "test-bucket",
		UploadPrefix:     "it-uploads",
		PutPresignExpire: 15 * time.Minute,
		Now:              func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewUploadSessionService() error = %v", err)
	}

	created, err := uploadSessionSvc.Create(ctx, publishingapp.CreateUploadSessionInput{
		OwnerUserID:       userID,
		ContextType:       "official_today",
		OfficialKeywordID: &keywordID,
		BizDate:           &bizDate,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	imageBody := buildJPEG(t, 1200, 1600)
	presigned, err := uploadSessionSvc.PresignBatch(ctx, publishingapp.PresignBatchInput{
		OwnerUserID: userID,
		SessionID:   created.SessionID,
		Items: []publishingapp.PresignBatchItemInput{
			{
				ClientImageID:      "cover-1",
				ImageContentType:   "image/jpeg",
				ImageContentLength: int64(len(imageBody)),
			},
		},
	})
	if err != nil {
		t.Fatalf("PresignBatch() error = %v", err)
	}

	target := presigned.Items[0].ImageUpload
	if err := storage.Put(ctx, "test-bucket", target.ObjectKey, imageBody, "image/jpeg"); err != nil {
		t.Fatalf("memory storage Put() error = %v", err)
	}

	_, err = uploadSessionSvc.CompleteBatch(ctx, publishingapp.CompleteBatchInput{
		OwnerUserID: userID,
		SessionID:   created.SessionID,
		Items: []publishingapp.CompleteBatchItemInput{
			{
				ItemID:       presigned.Items[0].ItemID,
				ImageEtag:    "\"wrong-etag\"",
				ImageWidth:   1200,
				ImageHeight:  1600,
				DisplayOrder: 1,
				IsCover:      true,
			},
		},
	})
	if !errors.Is(err, common.ErrConflict) {
		t.Fatalf("CompleteBatch() error = %v, want conflict", err)
	}
}

func TestOfficialUploadPublishFlow_PresignBatchIsIdempotentForPendingItem(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	storage := newMemoryObjectStorage()
	now := time.Now().UTC().Add(-2 * time.Second)

	userID := uuid.MustParse("41111111-1111-1111-1111-111111111111")
	keywordID := uuid.MustParse("42222222-2222-2222-2222-222222222222")
	bizDate := dateOnly(now)

	seedUser(t, ctx, pool, userID, "idempotent-user@example.com", "Idempotent User")
	seedOfficialKeyword(t, ctx, pool, keywordID, "晴朗", bizDate)
	officialCatalog := officialapp.NewCatalogService(officialrepo.NewRepository(officialdb.New(pool)), func() time.Time { return now })

	publishingSvc := publishingapp.NewService(pool, officialCatalog, func() time.Time { return now })
	uploadSessionSvc, err := publishingapp.NewUploadSessionService(pool, publishingSvc, storage, publishingapp.UploadSessionServiceConfig{
		Bucket:           "test-bucket",
		UploadPrefix:     "it-uploads",
		PutPresignExpire: 15 * time.Minute,
		Now:              func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("NewUploadSessionService() error = %v", err)
	}

	created, err := uploadSessionSvc.Create(ctx, publishingapp.CreateUploadSessionInput{
		OwnerUserID:       userID,
		ContextType:       "official_today",
		OfficialKeywordID: &keywordID,
		BizDate:           &bizDate,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	imageBody := buildJPEG(t, 1200, 1600)
	first, err := uploadSessionSvc.PresignBatch(ctx, publishingapp.PresignBatchInput{
		OwnerUserID: userID,
		SessionID:   created.SessionID,
		Items: []publishingapp.PresignBatchItemInput{
			{
				ClientImageID:      "cover-1",
				ImageContentType:   "image/jpeg",
				ImageContentLength: int64(len(imageBody)),
			},
		},
	})
	if err != nil {
		t.Fatalf("first PresignBatch() error = %v", err)
	}

	second, err := uploadSessionSvc.PresignBatch(ctx, publishingapp.PresignBatchInput{
		OwnerUserID: userID,
		SessionID:   created.SessionID,
		Items: []publishingapp.PresignBatchItemInput{
			{
				ClientImageID:      "cover-1",
				ImageContentType:   "image/jpeg",
				ImageContentLength: int64(len(imageBody)),
			},
		},
	})
	if err != nil {
		t.Fatalf("second PresignBatch() error = %v", err)
	}

	if len(first.Items) != 1 || len(second.Items) != 1 {
		t.Fatalf("PresignBatch() items = (%d, %d), want (1, 1)", len(first.Items), len(second.Items))
	}
	if second.Items[0].ItemID != first.Items[0].ItemID {
		t.Fatalf("second item id = %s, want %s", second.Items[0].ItemID, first.Items[0].ItemID)
	}
	if second.Items[0].ImageID != first.Items[0].ImageID {
		t.Fatalf("second image id = %s, want %s", second.Items[0].ImageID, first.Items[0].ImageID)
	}
	if second.Items[0].ImageUpload.ObjectKey != first.Items[0].ImageUpload.ObjectKey {
		t.Fatalf("second object key = %q, want %q", second.Items[0].ImageUpload.ObjectKey, first.Items[0].ImageUpload.ObjectKey)
	}

	var mediaAssetCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM media_assets`).Scan(&mediaAssetCount); err != nil {
		t.Fatalf("count media_assets: %v", err)
	}
	if mediaAssetCount != 1 {
		t.Fatalf("media asset count = %d, want 1", mediaAssetCount)
	}
}

func TestOfficialReadModelLazyAssignmentOnlyForTodayAndYesterday(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	now := time.Date(2026, time.April, 15, 9, 0, 0, 0, time.UTC)
	futureDate := dateOnly(now.AddDate(0, 0, 7))

	keywordID := uuid.MustParse("52222222-2222-2222-2222-222222222222")
	seedOfficialKeyword(t, ctx, pool, keywordID, "鏅存湕", futureDate)

	officialCatalog := officialapp.NewCatalogService(officialrepo.NewRepository(officialdb.New(pool)), func() time.Time { return now })
	readSvc := readmodelapp.NewService(
		readmodelrepo.NewRepository(readmodeldb.New(pool)),
		platformoss.NewURLResolver(platformoss.URLResolverConfig{
			PublicBaseURL: "https://cdn.test.local",
		}),
		officialCatalog,
		func() time.Time { return now },
	)

	if _, err := readSvc.GetOfficialHome(ctx, &futureDate); !errors.Is(err, common.ErrNotFound) {
		t.Fatalf("GetOfficialHome(future) error = %v, want not found", err)
	}
	if _, err := readSvc.ListOfficialDateUploads(ctx, futureDate, "latest", 20, nil, false, nil); !errors.Is(err, common.ErrNotFound) {
		t.Fatalf("ListOfficialDateUploads(future) error = %v, want not found", err)
	}

	var assignmentCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM daily_keyword_assignments WHERE biz_date IN ($1, $2)`, futureDate, futureDate.AddDate(0, 0, -1)).Scan(&assignmentCount); err != nil {
		t.Fatalf("count daily_keyword_assignments: %v", err)
	}
	if assignmentCount != 0 {
		t.Fatalf("future/day-before assignments = %d, want 0", assignmentCount)
	}
}
