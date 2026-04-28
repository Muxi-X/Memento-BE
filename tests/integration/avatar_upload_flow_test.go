package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	profileapp "cixing/internal/modules/profile/application"
	profiledb "cixing/internal/modules/profile/infra/db/gen"
	profilerepo "cixing/internal/modules/profile/infra/db/repo"
	readmodelapp "cixing/internal/modules/readmodel/application"
	readmodeldb "cixing/internal/modules/readmodel/infra/db/gen"
	readmodelrepo "cixing/internal/modules/readmodel/infra/db/repo"
	platformoss "cixing/internal/platform/oss"
	"cixing/internal/shared/common"
)

func TestAvatarUploadFlowCompletesAndRefreshesProfile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	storage := newMemoryObjectStorage()
	now := time.Now().UTC()

	userID := uuid.MustParse("91111111-1111-1111-1111-111111111111")
	seedUser(t, ctx, pool, userID, "avatar-user@example.com", "Avatar User")

	resolver := platformoss.NewURLResolver(platformoss.URLResolverConfig{
		PublicBaseURL: "https://cdn.test.local",
	})
	avatarSvc := newAvatarUploadServiceForTest(t, pool, storage, resolver, now)
	profileSvc := profileapp.NewService(profilerepo.NewRepository(profiledb.New(pool)), resolver)
	readmodelSvc := readmodelapp.NewService(readmodelrepo.NewRepository(readmodeldb.New(pool)), resolver, nil, nil)

	created, err := avatarSvc.Create(ctx, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Status != "created" {
		t.Fatalf("Create() status = %q, want created", created.Status)
	}

	imageBody := buildJPEG(t, 512, 512)
	presigned, err := avatarSvc.PresignImage(ctx, profileapp.PresignAvatarImageInput{
		UserID:             userID,
		SessionID:          created.SessionID,
		ImageContentType:   "image/jpeg",
		ImageContentLength: int64(len(imageBody)),
	})
	if err != nil {
		t.Fatalf("PresignImage() error = %v", err)
	}
	if presigned.Status != "presigned" {
		t.Fatalf("PresignImage() status = %q, want presigned", presigned.Status)
	}
	if !strings.Contains(presigned.ImageUpload.ObjectKey, "/avatars/"+userID.String()+"/"+created.SessionID.String()+"/image.jpg") {
		t.Fatalf("avatar object key = %q, want avatars path", presigned.ImageUpload.ObjectKey)
	}

	reused, err := avatarSvc.PresignImage(ctx, profileapp.PresignAvatarImageInput{
		UserID:             userID,
		SessionID:          created.SessionID,
		ImageContentType:   "image/jpeg",
		ImageContentLength: int64(len(imageBody)),
	})
	if err != nil {
		t.Fatalf("PresignImage(reuse) error = %v", err)
	}
	if reused.ImageID != presigned.ImageID || reused.ImageUpload.ObjectKey != presigned.ImageUpload.ObjectKey {
		t.Fatalf("PresignImage(reuse) = (%s, %s), want (%s, %s)", reused.ImageID, reused.ImageUpload.ObjectKey, presigned.ImageID, presigned.ImageUpload.ObjectKey)
	}

	_, err = avatarSvc.PresignImage(ctx, profileapp.PresignAvatarImageInput{
		UserID:             userID,
		SessionID:          created.SessionID,
		ImageContentType:   "image/jpeg",
		ImageContentLength: int64(len(imageBody)) + 1,
	})
	if !errors.Is(err, common.ErrConflict) {
		t.Fatalf("PresignImage(changed size) error = %v, want conflict", err)
	}

	if err := storage.Put(ctx, "test-bucket", presigned.ImageUpload.ObjectKey, imageBody, "image/jpeg"); err != nil {
		t.Fatalf("storage.Put() error = %v", err)
	}
	etag := mustHeadObjectETag(t, ctx, storage, "test-bucket", presigned.ImageUpload.ObjectKey)

	completed, err := avatarSvc.Complete(ctx, profileapp.CompleteAvatarUploadInput{
		UserID:      userID,
		SessionID:   created.SessionID,
		ImageETag:   etag,
		ImageWidth:  512,
		ImageHeight: 512,
	})
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if completed.Status != "completed" {
		t.Fatalf("Complete() status = %q, want completed", completed.Status)
	}
	assertAvatarURL(t, completed.Profile.AvatarURL, presigned.ImageUpload.ObjectKey)

	var currentAvatarID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT current_avatar_asset_id FROM user_profiles WHERE user_id = $1`, userID).Scan(&currentAvatarID); err != nil {
		t.Fatalf("query current avatar id: %v", err)
	}
	if currentAvatarID != presigned.ImageID {
		t.Fatalf("current_avatar_asset_id = %s, want %s", currentAvatarID, presigned.ImageID)
	}

	settings, err := profileSvc.GetSettings(ctx, userID)
	if err != nil {
		t.Fatalf("GetSettings() error = %v", err)
	}
	assertAvatarURL(t, settings.Profile.AvatarURL, presigned.ImageUpload.ObjectKey)

	home, err := readmodelSvc.GetMeHome(ctx, userID)
	if err != nil {
		t.Fatalf("GetMeHome() error = %v", err)
	}
	assertAvatarURL(t, home.AvatarURL, presigned.ImageUpload.ObjectKey)
}

func TestAvatarUploadCompleteRejectsMissingObjectWithoutUpdatingProfile(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	storage := newMemoryObjectStorage()
	now := time.Now().UTC()

	userID := uuid.MustParse("92222222-2222-2222-2222-222222222222")
	seedUser(t, ctx, pool, userID, "avatar-missing@example.com", "Missing Avatar")

	resolver := platformoss.NewURLResolver(platformoss.URLResolverConfig{
		PublicBaseURL: "https://cdn.test.local",
	})
	avatarSvc := newAvatarUploadServiceForTest(t, pool, storage, resolver, now)

	created, err := avatarSvc.Create(ctx, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	imageBody := buildJPEG(t, 320, 320)
	if _, err := avatarSvc.PresignImage(ctx, profileapp.PresignAvatarImageInput{
		UserID:             userID,
		SessionID:          created.SessionID,
		ImageContentType:   "image/jpeg",
		ImageContentLength: int64(len(imageBody)),
	}); err != nil {
		t.Fatalf("PresignImage() error = %v", err)
	}

	_, err = avatarSvc.Complete(ctx, profileapp.CompleteAvatarUploadInput{
		UserID:      userID,
		SessionID:   created.SessionID,
		ImageETag:   "missing",
		ImageWidth:  320,
		ImageHeight: 320,
	})
	if !errors.Is(err, common.ErrConflict) {
		t.Fatalf("Complete(missing object) error = %v, want conflict", err)
	}

	var avatarCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM user_profiles WHERE user_id = $1 AND current_avatar_asset_id IS NOT NULL`, userID).Scan(&avatarCount); err != nil {
		t.Fatalf("query avatar count: %v", err)
	}
	if avatarCount != 0 {
		t.Fatalf("current avatar set after failed complete, count = %d", avatarCount)
	}
}

func TestAvatarUploadCompleteRetrySucceedsAfterCompletedSessionExpires(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	storage := newMemoryObjectStorage()
	now := time.Now().UTC()
	currentNow := now

	userID := uuid.MustParse("94444444-4444-4444-4444-444444444444")
	seedUser(t, ctx, pool, userID, "avatar-retry@example.com", "Retry Avatar")

	resolver := platformoss.NewURLResolver(platformoss.URLResolverConfig{
		PublicBaseURL: "https://cdn.test.local",
	})
	avatarSvc, err := profileapp.NewAvatarUploadService(pool, storage, resolver, profileapp.AvatarUploadServiceConfig{
		Bucket:           "test-bucket",
		UploadPrefix:     "uploads/",
		PutPresignExpire: 15 * time.Minute,
		Now: func() time.Time {
			return currentNow
		},
	})
	if err != nil {
		t.Fatalf("NewAvatarUploadService() error = %v", err)
	}

	created, err := avatarSvc.Create(ctx, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	imageBody := buildJPEG(t, 256, 256)
	presigned, err := avatarSvc.PresignImage(ctx, profileapp.PresignAvatarImageInput{
		UserID:             userID,
		SessionID:          created.SessionID,
		ImageContentType:   "image/jpeg",
		ImageContentLength: int64(len(imageBody)),
	})
	if err != nil {
		t.Fatalf("PresignImage() error = %v", err)
	}
	if err := storage.Put(ctx, "test-bucket", presigned.ImageUpload.ObjectKey, imageBody, "image/jpeg"); err != nil {
		t.Fatalf("storage.Put() error = %v", err)
	}
	etag := mustHeadObjectETag(t, ctx, storage, "test-bucket", presigned.ImageUpload.ObjectKey)
	completeInput := profileapp.CompleteAvatarUploadInput{
		UserID:      userID,
		SessionID:   created.SessionID,
		ImageETag:   etag,
		ImageWidth:  256,
		ImageHeight: 256,
	}

	completed, err := avatarSvc.Complete(ctx, completeInput)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if completed.Status != "completed" {
		t.Fatalf("Complete() status = %q, want completed", completed.Status)
	}
	assertAvatarURL(t, completed.Profile.AvatarURL, presigned.ImageUpload.ObjectKey)

	currentNow = now.Add(49 * time.Hour)
	retried, err := avatarSvc.Complete(ctx, completeInput)
	if err != nil {
		t.Fatalf("Complete(retry after expiry) error = %v", err)
	}
	if retried.Status != "completed" {
		t.Fatalf("Complete(retry after expiry) status = %q, want completed", retried.Status)
	}
	assertAvatarURL(t, retried.Profile.AvatarURL, presigned.ImageUpload.ObjectKey)

	var status string
	var currentAvatarID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT aus.status::text, up.current_avatar_asset_id
FROM avatar_upload_sessions aus
JOIN user_profiles up ON up.user_id = aus.user_id
WHERE aus.id = $1`, created.SessionID).Scan(&status, &currentAvatarID); err != nil {
		t.Fatalf("query avatar completion state: %v", err)
	}
	if status != "completed" {
		t.Fatalf("avatar session status = %q, want completed", status)
	}
	if currentAvatarID != presigned.ImageID {
		t.Fatalf("current_avatar_asset_id = %s, want %s", currentAvatarID, presigned.ImageID)
	}
}

func TestAvatarUploadCompleteRejectsExpiredSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	storage := newMemoryObjectStorage()
	now := time.Now().UTC()
	currentNow := now

	userID := uuid.MustParse("93333333-3333-3333-3333-333333333333")
	seedUser(t, ctx, pool, userID, "avatar-expired@example.com", "Expired Avatar")

	resolver := platformoss.NewURLResolver(platformoss.URLResolverConfig{
		PublicBaseURL: "https://cdn.test.local",
	})
	avatarSvc, err := profileapp.NewAvatarUploadService(pool, storage, resolver, profileapp.AvatarUploadServiceConfig{
		Bucket:           "test-bucket",
		UploadPrefix:     "uploads/",
		PutPresignExpire: 15 * time.Minute,
		Now: func() time.Time {
			return currentNow
		},
	})
	if err != nil {
		t.Fatalf("NewAvatarUploadService() error = %v", err)
	}

	created, err := avatarSvc.Create(ctx, userID)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	imageBody := buildJPEG(t, 320, 320)
	presigned, err := avatarSvc.PresignImage(ctx, profileapp.PresignAvatarImageInput{
		UserID:             userID,
		SessionID:          created.SessionID,
		ImageContentType:   "image/jpeg",
		ImageContentLength: int64(len(imageBody)),
	})
	if err != nil {
		t.Fatalf("PresignImage() error = %v", err)
	}
	if err := storage.Put(ctx, "test-bucket", presigned.ImageUpload.ObjectKey, imageBody, "image/jpeg"); err != nil {
		t.Fatalf("storage.Put() error = %v", err)
	}
	etag := mustHeadObjectETag(t, ctx, storage, "test-bucket", presigned.ImageUpload.ObjectKey)

	currentNow = now.Add(49 * time.Hour)
	_, err = avatarSvc.Complete(ctx, profileapp.CompleteAvatarUploadInput{
		UserID:      userID,
		SessionID:   created.SessionID,
		ImageETag:   etag,
		ImageWidth:  320,
		ImageHeight: 320,
	})
	if !errors.Is(err, profileapp.ErrAvatarUploadExpired) {
		t.Fatalf("Complete(expired) error = %v, want expired", err)
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status::text FROM avatar_upload_sessions WHERE id = $1`, created.SessionID).Scan(&status); err != nil {
		t.Fatalf("query avatar session status: %v", err)
	}
	if status != "expired" {
		t.Fatalf("avatar session status = %q, want expired", status)
	}
}

func newAvatarUploadServiceForTest(t *testing.T, pool *pgxpool.Pool, storage common.ObjectStorage, resolver *platformoss.URLResolver, now time.Time) *profileapp.AvatarUploadService {
	t.Helper()

	svc, err := profileapp.NewAvatarUploadService(pool, storage, resolver, profileapp.AvatarUploadServiceConfig{
		Bucket:           "test-bucket",
		UploadPrefix:     "uploads/",
		PutPresignExpire: 15 * time.Minute,
		Now: func() time.Time {
			return now
		},
	})
	if err != nil {
		t.Fatalf("NewAvatarUploadService() error = %v", err)
	}
	return svc
}

func assertAvatarURL(t *testing.T, got *string, objectKey string) {
	t.Helper()

	if got == nil {
		t.Fatalf("avatar URL is nil")
	}
	if !strings.Contains(*got, objectKey) {
		t.Fatalf("avatar URL = %q, want object key %q", *got, objectKey)
	}
	if !strings.Contains(*got, "x-oss-process=style/square_small") {
		t.Fatalf("avatar URL = %q, want square_small style", *got)
	}
}
