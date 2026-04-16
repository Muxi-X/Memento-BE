package integration

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	officialapp "cixing/internal/modules/official/application"
	officialdb "cixing/internal/modules/official/infra/db/gen"
	officialrepo "cixing/internal/modules/official/infra/db/repo"
	publishingapp "cixing/internal/modules/publishing/application"
	"cixing/internal/shared/common"
)

const testDatabaseURLEnv = "TEST_DATABASE_URL"

func openIntegrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	baseURL := strings.TrimSpace(os.Getenv(testDatabaseURLEnv))
	if baseURL == "" {
		t.Skipf("%s is not set", testDatabaseURLEnv)
	}

	ctx := context.Background()
	adminURL := rewriteDatabaseURL(t, baseURL, "postgres")
	adminConn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect admin db: %v", err)
	}
	defer adminConn.Close(ctx)

	dbName := fmt.Sprintf("cixing_it_%d", time.Now().UnixNano())
	if _, err := adminConn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		t.Fatalf("create test database: %v", err)
	}

	pool, err := pgxpool.New(ctx, rewriteDatabaseURL(t, baseURL, dbName))
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}

	applyIntegrationMigrations(ctx, t, pool)

	t.Cleanup(func() {
		pool.Close()

		adminConn, err := pgx.Connect(context.Background(), adminURL)
		if err != nil {
			t.Fatalf("reconnect admin db for cleanup: %v", err)
		}
		defer adminConn.Close(context.Background())

		if _, err := adminConn.Exec(context.Background(), "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()", dbName); err != nil {
			t.Fatalf("terminate test db connections: %v", err)
		}
		if _, err := adminConn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()); err != nil {
			t.Fatalf("drop test database: %v", err)
		}
	})

	return pool
}

func applyIntegrationMigrations(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	migrationsDir := filepath.Join("..", "..", "db", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		pool.Close()
		t.Fatalf("read migrations dir: %v", err)
	}

	var migrationFiles []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		migrationFiles = append(migrationFiles, filepath.Join(migrationsDir, entry.Name()))
	}
	sort.Strings(migrationFiles)

	for _, migrationPath := range migrationFiles {
		migrationSQL, err := os.ReadFile(migrationPath)
		if err != nil {
			pool.Close()
			t.Fatalf("read migration %s: %v", filepath.Base(migrationPath), err)
		}
		if _, err := pool.Exec(ctx, string(migrationSQL)); err != nil {
			pool.Close()
			t.Fatalf("apply migration %s: %v", filepath.Base(migrationPath), err)
		}
	}
}

func rewriteDatabaseURL(t *testing.T, rawURL, dbName string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse database url: %v", err)
	}
	parsed.Path = "/" + dbName
	return parsed.String()
}

func seedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, email string, nickname string) {
	t.Helper()

	if _, err := pool.Exec(ctx, `INSERT INTO users (id) VALUES ($1)`, userID); err != nil {
		t.Fatalf("insert users: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_email_identities (user_id, email, email_verified) VALUES ($1, $2, TRUE)`, userID, email); err != nil {
		t.Fatalf("insert user_email_identities: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_profiles (user_id, nickname, bio) VALUES ($1, $2, '')`, userID, nickname); err != nil {
		t.Fatalf("insert user_profiles: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_settings (user_id, public_pool_enabled, privacy_version, reaction_notification_enabled, creation_reminder_enabled) VALUES ($1, TRUE, 0, TRUE, TRUE)`, userID); err != nil {
		t.Fatalf("insert user_settings: %v", err)
	}
}

func seedOfficialKeyword(t *testing.T, ctx context.Context, pool *pgxpool.Pool, keywordID uuid.UUID, text string, bizDate time.Time) {
	t.Helper()
	_ = bizDate

	if _, err := pool.Exec(ctx, `INSERT INTO official_keywords (id, text, category, is_active, display_order) VALUES ($1, $2, 'emotion', TRUE, 999)`, keywordID, text); err != nil {
		t.Fatalf("insert official_keywords: %v", err)
	}
}

func seedDailyKeywordAssignment(t *testing.T, ctx context.Context, pool *pgxpool.Pool, keywordID uuid.UUID, bizDate time.Time) {
	t.Helper()

	bizDate = dateOnly(bizDate)
	if _, err := pool.Exec(ctx, `
INSERT INTO daily_keyword_assignments (biz_date, keyword_id)
VALUES ($1, $2)
ON CONFLICT (biz_date) DO UPDATE SET keyword_id = EXCLUDED.keyword_id
`, bizDate, keywordID); err != nil {
		t.Fatalf("upsert daily_keyword_assignments: %v", err)
	}
}

func seedOfficialKeywordAsDaily(t *testing.T, ctx context.Context, pool *pgxpool.Pool, keywordID uuid.UUID, text string, bizDate time.Time) {
	t.Helper()

	seedOfficialKeyword(t, ctx, pool, keywordID, text, bizDate)
	seedDailyKeywordAssignment(t, ctx, pool, keywordID, bizDate)
}

func assertJobCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want int) {
	t.Helper()

	var got int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM jobs`).Scan(&got); err != nil {
		t.Fatalf("query total jobs: %v", err)
	}
	if got != want {
		t.Fatalf("total job count = %d, want %d", got, want)
	}
}

func createVisibleOfficialUploadForTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, storage *memoryObjectStorage, ownerUserID, keywordID uuid.UUID, bizDate, now time.Time) uuid.UUID {
	t.Helper()

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
		OwnerUserID:       ownerUserID,
		ContextType:       "official_today",
		OfficialKeywordID: &keywordID,
		BizDate:           &bizDate,
	})
	if err != nil {
		t.Fatalf("upload Create() error = %v", err)
	}

	imageBody := buildJPEG(t, 1200, 1600)
	presigned, err := uploadSessionSvc.PresignBatch(ctx, publishingapp.PresignBatchInput{
		OwnerUserID: ownerUserID,
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
		t.Fatalf("upload PresignBatch() error = %v", err)
	}
	if err := storage.Put(ctx, "test-bucket", presigned.Items[0].ImageUpload.ObjectKey, imageBody, "image/jpeg"); err != nil {
		t.Fatalf("upload image Put() error = %v", err)
	}
	imageETag := mustHeadObjectETag(t, ctx, storage, "test-bucket", presigned.Items[0].ImageUpload.ObjectKey)
	if _, err := uploadSessionSvc.CompleteBatch(ctx, publishingapp.CompleteBatchInput{
		OwnerUserID: ownerUserID,
		SessionID:   created.SessionID,
		Items: []publishingapp.CompleteBatchItemInput{
			{
				ItemID:       presigned.Items[0].ItemID,
				ImageEtag:    imageETag,
				ImageWidth:   1200,
				ImageHeight:  1600,
				DisplayOrder: 1,
				IsCover:      true,
				Title:        strPtr("cover"),
				Note:         strPtr("note"),
			},
		},
	}); err != nil {
		t.Fatalf("upload CompleteBatch() error = %v", err)
	}
	committed, err := uploadSessionSvc.Commit(ctx, publishingapp.CommitUploadSessionInput{
		OwnerUserID: ownerUserID,
		SessionID:   created.SessionID,
	})
	if err != nil {
		t.Fatalf("upload Commit() error = %v", err)
	}

	return committed.UploadID
}

func buildJPEG(t *testing.T, width, height int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 255) / width),
				G: uint8((y * 255) / height),
				B: 180,
				A: 255,
			})
		}
	}

	buf := bytes.NewBuffer(nil)
	if err := jpeg.Encode(buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func mustHeadObjectETag(t *testing.T, ctx context.Context, storage common.ObjectStorage, bucket string, key string) string {
	t.Helper()

	exists, _, etag, _, err := storage.Head(ctx, bucket, key)
	if err != nil {
		t.Fatalf("storage.Head(%s) error = %v", key, err)
	}
	if !exists {
		t.Fatalf("storage.Head(%s) exists = false, want true", key)
	}
	return etag
}

func strPtr(v string) *string {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

func intPtr(v int) *int {
	return &v
}

func uuidPtr(v uuid.UUID) *uuid.UUID {
	return &v
}

func dateOnly(t time.Time) time.Time {
	return common.NormalizeBizDate(t)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

type memoryObjectStorage struct {
	mu      sync.Mutex
	objects map[string]memoryObject
}

type memoryObject struct {
	body        []byte
	contentType string
}

func newMemoryObjectStorage() *memoryObjectStorage {
	return &memoryObjectStorage{
		objects: make(map[string]memoryObject),
	}
}

func (s *memoryObjectStorage) PresignPut(_ context.Context, bucket, key, _ string, _ int64, expires time.Duration) (*common.PresignResult, error) {
	return &common.PresignResult{
		Method:    "PUT",
		URL:       "memory://" + bucket + "/" + strings.TrimLeft(key, "/"),
		ExpiresAt: time.Now().UTC().Add(expires),
	}, nil
}

func (s *memoryObjectStorage) PresignGet(_ context.Context, bucket, key string, expires time.Duration) (*common.PresignResult, error) {
	return &common.PresignResult{
		Method:    "GET",
		URL:       "memory://" + bucket + "/" + strings.TrimLeft(key, "/"),
		ExpiresAt: time.Now().UTC().Add(expires),
	}, nil
}

func (s *memoryObjectStorage) Get(_ context.Context, bucket, key string) ([]byte, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	obj, ok := s.objects[s.objectID(bucket, key)]
	if !ok {
		return nil, "", common.ErrNotFound
	}
	body := make([]byte, len(obj.body))
	copy(body, obj.body)
	return body, obj.contentType, nil
}

func (s *memoryObjectStorage) Put(_ context.Context, bucket, key string, body []byte, contentType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	copied := make([]byte, len(body))
	copy(copied, body)
	s.objects[s.objectID(bucket, key)] = memoryObject{
		body:        copied,
		contentType: contentType,
	}
	return nil
}

func (s *memoryObjectStorage) Head(_ context.Context, bucket, key string) (exists bool, size int64, etag string, contentType string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	obj, ok := s.objects[s.objectID(bucket, key)]
	if !ok {
		return false, 0, "", "", nil
	}
	return true, int64(len(obj.body)), "\"memory-etag\"", obj.contentType, nil
}

func (s *memoryObjectStorage) Delete(_ context.Context, bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.objects, s.objectID(bucket, key))
	return nil
}

func (s *memoryObjectStorage) objectID(bucket, key string) string {
	return bucket + "::" + strings.TrimLeft(key, "/")
}
