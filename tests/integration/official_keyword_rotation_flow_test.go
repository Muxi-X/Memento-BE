package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	officialapp "cixing/internal/modules/official/application"
	dofficial "cixing/internal/modules/official/domain"
	officialdb "cixing/internal/modules/official/infra/db/gen"
	officialrepo "cixing/internal/modules/official/infra/db/repo"
	publishingapp "cixing/internal/modules/publishing/application"
	readmodelapp "cixing/internal/modules/readmodel/application"
	readmodeldb "cixing/internal/modules/readmodel/infra/db/gen"
	readmodelrepo "cixing/internal/modules/readmodel/infra/db/repo"
	platformoss "cixing/internal/platform/oss"
	"cixing/internal/platform/postgres"
	"cixing/internal/shared/common"
)

func TestOfficialKeywordRotationConcurrentIdempotentAndRestartSafe(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	effective := dateOnly(time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC))
	target := effective.AddDate(0, 0, 1)
	now := target.Add(12 * time.Hour)

	catalog1, _ := newTestCatalogService(pool, func() time.Time { return now })
	catalog2, _ := newTestCatalogService(pool, func() time.Time { return now })
	if err := catalog1.InitializeRotation(ctx, effective); err != nil {
		t.Fatalf("InitializeRotation() error = %v", err)
	}

	type result struct {
		assignment dofficial.DailyKeywordAssignment
		err        error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, catalog := range []*officialapp.CatalogService{catalog1, catalog2} {
		catalog := catalog
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			assignment, err := catalog.EnsureDailyKeywordAssignment(ctx, target)
			results <- result{assignment: assignment, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var first result
	for got := range results {
		if got.err != nil {
			t.Fatalf("EnsureDailyKeywordAssignment(concurrent) error = %v", got.err)
		}
		if first.assignment.KeywordID == uuid.Nil {
			first = got
			continue
		}
		if got.assignment.KeywordID != first.assignment.KeywordID {
			t.Fatalf("concurrent keyword ids differ: %s and %s", first.assignment.KeywordID, got.assignment.KeywordID)
		}
	}

	var assignmentCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM daily_keyword_assignments WHERE biz_date = $1`, target).Scan(&assignmentCount); err != nil {
		t.Fatalf("count daily assignments: %v", err)
	}
	if assignmentCount != 1 {
		t.Fatalf("assignment count = %d, want 1", assignmentCount)
	}

	before := snapshotRotationQueue(t, ctx, pool)
	restartedCatalog, _ := newTestCatalogService(pool, func() time.Time { return now })
	replayed, err := restartedCatalog.EnsureDailyKeywordAssignment(ctx, target)
	if err != nil {
		t.Fatalf("EnsureDailyKeywordAssignment(restart replay) error = %v", err)
	}
	if replayed.KeywordID != first.assignment.KeywordID {
		t.Fatalf("restart replay keyword = %s, want %s", replayed.KeywordID, first.assignment.KeywordID)
	}
	after := snapshotRotationQueue(t, ctx, pool)
	if fmt.Sprint(before) != fmt.Sprint(after) {
		t.Fatalf("queue changed on replay: before=%v after=%v", before, after)
	}
}

func TestOfficialKeywordRotationRepeatDoesNotAdvanceQueue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	effective := dateOnly(time.Date(2026, time.January, 1, 10, 0, 0, 0, time.UTC))
	addOfficialKeywordsForRotationTest(t, ctx, pool, 26, 100)

	now := effective.AddDate(0, 0, 29).Add(12 * time.Hour)
	catalog, _ := newTestCatalogService(pool, func() time.Time { return now })
	if err := catalog.InitializeRotation(ctx, effective); err != nil {
		t.Fatalf("InitializeRotation() error = %v", err)
	}
	if _, err := catalog.EnsureDailyKeywordAssignment(ctx, effective.AddDate(0, 0, 28)); err != nil {
		t.Fatalf("ensure first 29 days: %v", err)
	}

	beforeQueue := snapshotRotationQueue(t, ctx, pool)
	var beforeNextPosition int64
	if err := pool.QueryRow(ctx, `SELECT next_queue_position FROM official_keyword_rotation_state WHERE singleton`).Scan(&beforeNextPosition); err != nil {
		t.Fatalf("query next queue position: %v", err)
	}

	repeatDay := effective.AddDate(0, 0, 29)
	if _, err := catalog.EnsureDailyKeywordAssignment(ctx, repeatDay); err != nil {
		t.Fatalf("EnsureDailyKeywordAssignment(repeat day) error = %v", err)
	}
	d23, err := testAssignmentKeywordID(ctx, pool, effective.AddDate(0, 0, 22))
	if err != nil {
		t.Fatalf("get day 23 assignment: %v", err)
	}
	d30, err := testAssignmentKeywordID(ctx, pool, repeatDay)
	if err != nil {
		t.Fatalf("get day 30 assignment: %v", err)
	}
	if d30 != d23 {
		t.Fatalf("day 30 keyword = %s, want day 23 keyword %s", d30, d23)
	}

	afterQueue := snapshotRotationQueue(t, ctx, pool)
	if fmt.Sprint(beforeQueue) != fmt.Sprint(afterQueue) {
		t.Fatalf("queue changed on repeat: before=%v after=%v", beforeQueue, afterQueue)
	}
	var afterNextPosition int64
	if err := pool.QueryRow(ctx, `SELECT next_queue_position FROM official_keyword_rotation_state WHERE singleton`).Scan(&afterNextPosition); err != nil {
		t.Fatalf("query next queue position after repeat: %v", err)
	}
	if afterNextPosition != beforeNextPosition {
		t.Fatalf("next queue position = %d, want unchanged %d", afterNextPosition, beforeNextPosition)
	}
}

func TestOfficialHomeLazyAssignmentUsesChronologicalOrder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	target := dateOnly(time.Date(2026, time.October, 2, 10, 0, 0, 0, time.UTC))
	now := target.Add(12 * time.Hour)
	catalog, _ := newTestCatalogService(pool, func() time.Time { return now })
	if err := catalog.InitializeRotation(ctx, target.AddDate(0, 0, -1)); err != nil {
		t.Fatalf("InitializeRotation() error = %v", err)
	}

	readService := readmodelapp.NewService(readmodelrepo.NewRepository(readmodeldb.New(pool), pool), platformoss.NewURLResolver(platformoss.URLResolverConfig{PublicBaseURL: "https://cdn.test.local"}), catalog, func() time.Time { return now })
	home, err := readService.GetOfficialHome(ctx, &target)
	if err != nil {
		t.Fatalf("GetOfficialHome() error = %v", err)
	}
	if home.Today.Keyword.ID == home.Yesterday.Keyword.ID {
		t.Fatalf("home keyword ids are equal: %s", home.Today.Keyword.ID)
	}

	rows, err := pool.Query(ctx, `
		SELECT id
		FROM official_keywords
		WHERE is_active = TRUE
		ORDER BY display_order ASC, id ASC
		LIMIT 2
	`)
	if err != nil {
		t.Fatalf("query first queue keywords: %v", err)
	}
	defer rows.Close()
	var expected []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan first queue keyword: %v", err)
		}
		expected = append(expected, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate first queue keywords: %v", err)
	}
	if len(expected) != 2 {
		t.Fatalf("first queue keyword count = %d, want 2", len(expected))
	}
	if home.Yesterday.Keyword.ID != expected[0] || home.Today.Keyword.ID != expected[1] {
		t.Fatalf("home assignments = yesterday:%s today:%s, want %s then %s", home.Yesterday.Keyword.ID, home.Today.Keyword.ID, expected[0], expected[1])
	}
}

func TestOfficialKeywordRotationTransactionRollback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	effective := dateOnly(time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC))
	catalog, _ := newTestCatalogService(pool, func() time.Time { return effective.Add(12 * time.Hour) })
	if err := catalog.InitializeRotation(ctx, effective); err != nil {
		t.Fatalf("InitializeRotation() error = %v", err)
	}

	var keywordID uuid.UUID
	var beforePosition int64
	if err := pool.QueryRow(ctx, `
		SELECT keyword_id, queue_position
		FROM official_keyword_rotation_queue
		ORDER BY queue_position
		LIMIT 1
	`).Scan(&keywordID, &beforePosition); err != nil {
		t.Fatalf("query first queue item: %v", err)
	}
	var beforeNextPosition int64
	if err := pool.QueryRow(ctx, `SELECT next_queue_position FROM official_keyword_rotation_state WHERE singleton`).Scan(&beforeNextPosition); err != nil {
		t.Fatalf("query next queue position: %v", err)
	}

	rollbackErr := errors.New("forced rollback")
	err := postgres.WithTx(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		repo := officialrepo.NewRepository(officialdb.New(tx))
		if _, err := repo.LockRotationState(ctx); err != nil {
			return err
		}
		if _, err := repo.MoveRotationKeywordToTail(ctx, keywordID); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("transaction error = %v, want forced rollback", err)
	}

	var afterPosition int64
	if err := pool.QueryRow(ctx, `SELECT queue_position FROM official_keyword_rotation_queue WHERE keyword_id = $1`, keywordID).Scan(&afterPosition); err != nil {
		t.Fatalf("query rolled back queue item: %v", err)
	}
	if afterPosition != beforePosition {
		t.Fatalf("queue position after rollback = %d, want %d", afterPosition, beforePosition)
	}
	var afterNextPosition int64
	if err := pool.QueryRow(ctx, `SELECT next_queue_position FROM official_keyword_rotation_state WHERE singleton`).Scan(&afterNextPosition); err != nil {
		t.Fatalf("query next queue position after rollback: %v", err)
	}
	if afterNextPosition != beforeNextPosition {
		t.Fatalf("next queue position after rollback = %d, want %d", afterNextPosition, beforeNextPosition)
	}
}

func newTestCatalogService(pool *pgxpool.Pool, now func() time.Time) (*officialapp.CatalogService, *officialrepo.Repository) {
	repo := officialrepo.NewRepository(officialdb.New(pool))
	return officialapp.NewCatalogService(pool, repo, now), repo
}

func addOfficialKeywordsForRotationTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, count int, firstDisplayOrder int) {
	t.Helper()
	for i := 0; i < count; i++ {
		if _, err := pool.Exec(ctx, `
			INSERT INTO official_keywords (id, text, category, is_active, display_order)
			VALUES ($1, $2, 'abstract', TRUE, $3)
		`, uuid.New(), fmt.Sprintf("rotation-extra-%02d", i+1), firstDisplayOrder+i); err != nil {
			t.Fatalf("insert rotation keyword %d: %v", i, err)
		}
	}
}

func snapshotRotationQueue(t *testing.T, ctx context.Context, pool *pgxpool.Pool) map[uuid.UUID]int64 {
	t.Helper()
	rows, err := pool.Query(ctx, `
		SELECT keyword_id, queue_position
		FROM official_keyword_rotation_queue
		ORDER BY queue_position
	`)
	if err != nil {
		t.Fatalf("query rotation queue: %v", err)
	}
	defer rows.Close()
	out := make(map[uuid.UUID]int64)
	for rows.Next() {
		var (
			keywordID uuid.UUID
			position  int64
		)
		if err := rows.Scan(&keywordID, &position); err != nil {
			t.Fatalf("scan rotation queue: %v", err)
		}
		out[keywordID] = position
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate rotation queue: %v", err)
	}
	return out
}

func testAssignmentKeywordID(ctx context.Context, pool *pgxpool.Pool, bizDate time.Time) (uuid.UUID, error) {
	var keywordID uuid.UUID
	err := pool.QueryRow(ctx, `SELECT keyword_id FROM daily_keyword_assignments WHERE biz_date = $1`, dateOnly(bizDate)).Scan(&keywordID)
	return keywordID, err
}
func TestOfficialPromptDrawResamplesAfterRotationLockWait(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	oldNow := time.Date(2026, time.September, 14, 15, 59, 59, 0, time.UTC) // 23:59:59 Asia/Shanghai
	oldBizDate := common.NormalizeBizDate(oldNow)
	clock := newTestClock(oldNow)
	catalog, _ := newTestCatalogService(pool, clock.Now)
	if err := catalog.InitializeRotation(ctx, oldBizDate); err != nil {
		t.Fatalf("InitializeRotation() error = %v", err)
	}

	userID := uuid.New()
	seedUser(t, ctx, pool, userID, "rotation-lock-wait@example.com", "rotation-lock-wait")
	promptSvc := officialapp.NewPromptService(pool, officialrepo.NewRepository(officialdb.New(pool)), catalog, clock.Now)

	lockTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin rotation lock tx: %v", err)
	}
	defer func() { _ = lockTx.Rollback(ctx) }()
	if _, err := lockTx.Exec(ctx, `SELECT 1 FROM official_keyword_rotation_state WHERE singleton FOR UPDATE`); err != nil {
		t.Fatalf("lock rotation state: %v", err)
	}

	type drawResult struct {
		out *officialapp.PromptOutput
		err error
	}
	resultCh := make(chan drawResult, 1)
	go func() {
		out, err := promptSvc.Draw(ctx, userID, uuid.New(), "intuition", &oldBizDate)
		resultCh <- drawResult{out: out, err: err}
	}()

	waitForLockWait(t, ctx, pool)
	clock.Set(oldNow.Add(2 * time.Second)) // 00:00:01 Asia/Shanghai next day
	if err := lockTx.Commit(ctx); err != nil {
		t.Fatalf("commit rotation lock tx: %v", err)
	}

	select {
	case result := <-resultCh:
		if !errors.Is(result.err, officialapp.ErrPromptDateChanged) {
			t.Fatalf("Draw(rotation lock crossed midnight) error = %v, want ErrPromptDateChanged", result.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Draw did not finish after rotation lock release")
	}
	if got := dailyPromptCount(t, ctx, pool, userID, oldBizDate); got != 0 {
		t.Fatalf("daily prompt count on old date = %d, want 0", got)
	}
}
func TestOfficialKeywordRotationMigrationDownAndUpPreserveHistory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	keywordID := uuid.New()
	bizDate := dateOnly(time.Date(2026, time.December, 1, 10, 0, 0, 0, time.UTC))
	seedOfficialKeywordAsDaily(t, ctx, pool, keywordID, "migration-history", bizDate)

	for _, name := range []string{
		"000006_official_keyword_rotation.down.sql",
		"000006_official_keyword_rotation.up.sql",
	} {
		migrationPath := filepath.Join("..", "..", "db", "migrations", name)
		migrationSQL, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx, string(migrationSQL)); err != nil {
			t.Fatalf("apply migration %s: %v", name, err)
		}
	}

	var gotKeywordID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT keyword_id FROM daily_keyword_assignments WHERE biz_date = $1`, bizDate).Scan(&gotKeywordID); err != nil {
		t.Fatalf("query preserved assignment: %v", err)
	}
	if gotKeywordID != keywordID {
		t.Fatalf("preserved assignment keyword = %s, want %s", gotKeywordID, keywordID)
	}

	var initialized bool
	if err := pool.QueryRow(ctx, `SELECT initialized FROM official_keyword_rotation_state WHERE singleton`).Scan(&initialized); err != nil {
		t.Fatalf("query rotation state after up: %v", err)
	}
	if initialized {
		t.Fatal("rotation state initialized after migration up, want explicit initialization boundary")
	}
}
func TestOfficialKeywordRotationAppliesKeywordChangeOnItsEffectiveDate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	effective := dateOnly(time.Date(2026, time.December, 10, 0, 0, 0, 0, time.UTC))
	addOfficialKeywordsForRotationTest(t, ctx, pool, 26, 100)

	now := effective.Add(12 * time.Hour)
	catalog, _ := newTestCatalogService(pool, func() time.Time { return now })
	if err := catalog.InitializeRotation(ctx, effective); err != nil {
		t.Fatalf("InitializeRotation() error = %v", err)
	}

	var keywordID uuid.UUID
	var text string
	var category string
	var displayOrder int32
	if err := pool.QueryRow(ctx, `
		SELECT id, text, category::text, display_order
		FROM official_keywords
		WHERE is_active = TRUE
		ORDER BY display_order ASC, id ASC
		LIMIT 1
	`).Scan(&keywordID, &text, &category, &displayOrder); err != nil {
		t.Fatalf("query keyword to deactivate: %v", err)
	}
	if _, err := catalog.ScheduleOfficialKeyword(ctx, dofficial.UpsertKeywordParams{
		ID:           keywordID,
		Text:         text,
		Category:     dofficial.KeywordCategory(category),
		IsActive:     false,
		DisplayOrder: &displayOrder,
	}); err != nil {
		t.Fatalf("ScheduleOfficialKeyword(deactivate) error = %v", err)
	}

	var active bool
	if err := pool.QueryRow(ctx, `SELECT is_active FROM official_keywords WHERE id = $1`, keywordID).Scan(&active); err != nil {
		t.Fatalf("query active before effective date: %v", err)
	}
	if !active {
		t.Fatal("keyword was deactivated before its effective date")
	}
	var appliedBefore bool
	if err := pool.QueryRow(ctx, `
		SELECT applied_at IS NOT NULL
		FROM official_keyword_changes
		WHERE keyword_id = $1 AND action = 'deactivate'
		ORDER BY id DESC
		LIMIT 1
	`, keywordID).Scan(&appliedBefore); err != nil {
		t.Fatalf("query pending change before effective date: %v", err)
	}
	if appliedBefore {
		t.Fatal("deactivation change applied before its effective date")
	}

	now = effective.AddDate(0, 0, 1).Add(12 * time.Hour)
	if _, err := catalog.EnsureDailyKeywordAssignment(ctx, effective.AddDate(0, 0, 1)); err != nil {
		t.Fatalf("EnsureDailyKeywordAssignment(effective date) error = %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT is_active FROM official_keywords WHERE id = $1`, keywordID).Scan(&active); err != nil {
		t.Fatalf("query active on effective date: %v", err)
	}
	if active {
		t.Fatal("keyword remained active on its effective date")
	}
	var appliedAfter bool
	if err := pool.QueryRow(ctx, `
		SELECT applied_at IS NOT NULL
		FROM official_keyword_changes
		WHERE keyword_id = $1 AND action = 'deactivate'
		ORDER BY id DESC
		LIMIT 1
	`, keywordID).Scan(&appliedAfter); err != nil {
		t.Fatalf("query applied change: %v", err)
	}
	if !appliedAfter {
		t.Fatal("deactivation change was not marked applied")
	}

	var positionBeforeReactivation int64
	if err := pool.QueryRow(ctx, `SELECT queue_position FROM official_keyword_rotation_queue WHERE keyword_id = $1`, keywordID).Scan(&positionBeforeReactivation); err != nil {
		t.Fatalf("query position before reactivation: %v", err)
	}
	if _, err := catalog.ScheduleOfficialKeyword(ctx, dofficial.UpsertKeywordParams{
		ID:           keywordID,
		Text:         text,
		Category:     dofficial.KeywordCategory(category),
		IsActive:     true,
		DisplayOrder: &displayOrder,
	}); err != nil {
		t.Fatalf("ScheduleOfficialKeyword(reactivate) error = %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT is_active FROM official_keywords WHERE id = $1`, keywordID).Scan(&active); err != nil {
		t.Fatalf("query active before reactivation date: %v", err)
	}
	if active {
		t.Fatal("keyword reactivated before its effective date")
	}

	now = effective.AddDate(0, 0, 2).Add(12 * time.Hour)
	if _, err := catalog.EnsureDailyKeywordAssignment(ctx, effective.AddDate(0, 0, 2)); err != nil {
		t.Fatalf("EnsureDailyKeywordAssignment(reactivation date) error = %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT is_active FROM official_keywords WHERE id = $1`, keywordID).Scan(&active); err != nil {
		t.Fatalf("query active after reactivation: %v", err)
	}
	if !active {
		t.Fatal("keyword remained inactive on reactivation date")
	}
	var queuePosition int64
	if err := pool.QueryRow(ctx, `SELECT queue_position FROM official_keyword_rotation_queue WHERE keyword_id = $1`, keywordID).Scan(&queuePosition); err != nil {
		t.Fatalf("query reactivated queue position: %v", err)
	}
	if queuePosition <= positionBeforeReactivation {
		t.Fatalf("reactivated queue position = %d, want greater than pre-reactivation %d", queuePosition, positionBeforeReactivation)
	}
}
func TestDailyPromptWithEmptyOfficialCatalogRemainsSuccessful(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	if _, err := pool.Exec(ctx, `UPDATE official_keywords SET is_active = FALSE`); err != nil {
		t.Fatalf("deactivate official keywords: %v", err)
	}
	userID := uuid.New()
	seedUser(t, ctx, pool, userID, "empty-official-catalog@example.com", "empty-official-catalog")

	now := time.Date(2026, time.December, 20, 10, 0, 0, 0, time.UTC)
	catalog, _ := newTestCatalogService(pool, func() time.Time { return now })
	promptSvc := officialapp.NewPromptService(pool, officialrepo.NewRepository(officialdb.New(pool)), catalog, func() time.Time { return now })
	state, err := promptSvc.GetDailyPrompt(ctx, userID)
	if err != nil {
		t.Fatalf("GetDailyPrompt(empty catalog) error = %v", err)
	}
	if state.KeywordID != nil || state.Selection != nil {
		t.Fatalf("GetDailyPrompt(empty catalog) = %+v, want null keyword and selection", state)
	}
	if !state.BizDate.Equal(common.NormalizeBizDate(now)) {
		t.Fatalf("GetDailyPrompt(empty catalog) biz_date = %s, want %s", state.BizDate, common.NormalizeBizDate(now))
	}
}
func TestOfficialPublishSessionDatePolicyAndCommitReadOnly(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	now := time.Date(2026, time.October, 10, 10, 0, 0, 0, time.UTC)
	today := common.NormalizeBizDate(now)
	past := today.AddDate(0, 0, -1)
	future := today.AddDate(0, 0, 1)
	ownerID := uuid.New()
	seedUser(t, ctx, pool, ownerID, "publish-date-policy@example.com", "publish-date-policy")

	pastKeywordID := uuid.New()
	futureKeywordID := uuid.New()
	todayKeywordID := uuid.New()
	seedOfficialKeywordAsDaily(t, ctx, pool, pastKeywordID, "past-keyword", past)
	seedOfficialKeywordAsDaily(t, ctx, pool, futureKeywordID, "future-keyword", future)
	seedOfficialKeywordAsDaily(t, ctx, pool, todayKeywordID, "today-keyword", today)

	svc := publishingapp.NewService(pool, nil, func() time.Time { return now })
	if _, err := svc.CreateOfficialSession(ctx, publishingapp.CreateOfficialSessionInput{
		OwnerUserID:       ownerID,
		OfficialKeywordID: pastKeywordID,
		BizDate:           past,
	}); err != nil {
		t.Fatalf("CreateOfficialSession(past existing) error = %v", err)
	}
	if _, err := svc.CreateOfficialSession(ctx, publishingapp.CreateOfficialSessionInput{
		OwnerUserID:       ownerID,
		OfficialKeywordID: pastKeywordID,
		BizDate:           past.AddDate(0, 0, -1),
	}); !errors.Is(err, common.ErrNotFound) {
		t.Fatalf("CreateOfficialSession(past missing) error = %v, want not found", err)
	}
	if _, err := svc.CreateOfficialSession(ctx, publishingapp.CreateOfficialSessionInput{
		OwnerUserID:       ownerID,
		OfficialKeywordID: futureKeywordID,
		BizDate:           future,
	}); !errors.Is(err, publishingapp.ErrInvalidUploadPublishInput) {
		t.Fatalf("CreateOfficialSession(future existing) error = %v, want invalid input", err)
	}

	var sessionCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM publish_sessions`).Scan(&sessionCount); err != nil {
		t.Fatalf("count publish sessions: %v", err)
	}
	if sessionCount != 1 {
		t.Fatalf("publish session count = %d, want 1", sessionCount)
	}

	todaySession, err := svc.CreateOfficialSession(ctx, publishingapp.CreateOfficialSessionInput{
		OwnerUserID:       ownerID,
		OfficialKeywordID: todayKeywordID,
		BizDate:           today,
	})
	if err != nil {
		t.Fatalf("CreateOfficialSession(today existing) error = %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM daily_keyword_assignments WHERE biz_date = $1`, today); err != nil {
		t.Fatalf("delete today's assignment: %v", err)
	}
	if _, err := svc.CommitSession(ctx, publishingapp.CommitSessionInput{
		OwnerUserID: ownerID,
		SessionID:   todaySession.ID,
	}); !errors.Is(err, common.ErrNotFound) {
		t.Fatalf("CommitSession(missing bound assignment) error = %v, want not found", err)
	}
	var assignmentCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM daily_keyword_assignments WHERE biz_date = $1`, today).Scan(&assignmentCount); err != nil {
		t.Fatalf("count today's assignments after commit: %v", err)
	}
	if assignmentCount != 0 {
		t.Fatalf("commit regenerated assignment count = %d, want 0", assignmentCount)
	}
}
func TestOfficialKeywordRotationInitializationConflictAndIdempotence(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	effective := dateOnly(time.Date(2026, time.December, 20, 0, 0, 0, 0, time.UTC))
	keywordID := uuid.New()
	seedOfficialKeywordAsDaily(t, ctx, pool, keywordID, "future-conflict", effective)

	catalog, _ := newTestCatalogService(pool, func() time.Time { return effective.AddDate(0, 0, -1).Add(12 * time.Hour) })
	if err := catalog.InitializeRotation(ctx, effective); !errors.Is(err, officialapp.ErrRotationStateInconsistent) {
		t.Fatalf("InitializeRotation(conflict) error = %v, want inconsistent state", err)
	}
	var initialized bool
	if err := pool.QueryRow(ctx, `SELECT initialized FROM official_keyword_rotation_state WHERE singleton`).Scan(&initialized); err != nil {
		t.Fatalf("query state after conflict: %v", err)
	}
	if initialized {
		t.Fatal("rotation state initialized after conflicting assignment")
	}

	if _, err := pool.Exec(ctx, `DELETE FROM daily_keyword_assignments WHERE biz_date = $1`, effective); err != nil {
		t.Fatalf("delete conflicting assignment: %v", err)
	}
	if err := catalog.InitializeRotation(ctx, effective); err != nil {
		t.Fatalf("InitializeRotation(clean) error = %v", err)
	}
	var beforeNextPosition int64
	if err := pool.QueryRow(ctx, `SELECT next_queue_position FROM official_keyword_rotation_state WHERE singleton`).Scan(&beforeNextPosition); err != nil {
		t.Fatalf("query initialized next position: %v", err)
	}
	if err := catalog.InitializeRotation(ctx, effective.AddDate(0, 0, 3)); err != nil {
		t.Fatalf("InitializeRotation(idempotent) error = %v", err)
	}
	var afterNextPosition int64
	if err := pool.QueryRow(ctx, `SELECT next_queue_position FROM official_keyword_rotation_state WHERE singleton`).Scan(&afterNextPosition); err != nil {
		t.Fatalf("query idempotent next position: %v", err)
	}
	if afterNextPosition != beforeNextPosition {
		t.Fatalf("idempotent initialization changed next position: got %d want %d", afterNextPosition, beforeNextPosition)
	}
}
func TestOfficialKeywordRotationRejectsInfeasibleDeactivation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	if _, err := pool.Exec(ctx, `UPDATE official_keywords SET is_active = FALSE`); err != nil {
		t.Fatalf("deactivate seed keywords: %v", err)
	}
	addOfficialKeywordsForRotationTest(t, ctx, pool, 7, 100)

	effective := dateOnly(time.Date(2026, time.December, 25, 0, 0, 0, 0, time.UTC))
	now := effective.Add(12 * time.Hour)
	catalog, _ := newTestCatalogService(pool, func() time.Time { return now })
	if err := catalog.InitializeRotation(ctx, effective); err != nil {
		t.Fatalf("InitializeRotation() error = %v", err)
	}

	var keywordID uuid.UUID
	var text string
	var category string
	var displayOrder int32
	if err := pool.QueryRow(ctx, `
		SELECT id, text, category::text, display_order
		FROM official_keywords
		WHERE is_active = TRUE
		ORDER BY display_order ASC, id ASC
		LIMIT 1
	`).Scan(&keywordID, &text, &category, &displayOrder); err != nil {
		t.Fatalf("query keyword to deactivate: %v", err)
	}
	if _, err := catalog.ScheduleOfficialKeyword(ctx, dofficial.UpsertKeywordParams{
		ID:           keywordID,
		Text:         text,
		Category:     dofficial.KeywordCategory(category),
		IsActive:     false,
		DisplayOrder: &displayOrder,
	}); !errors.Is(err, officialapp.ErrKeywordChangeNotFeasible) {
		t.Fatalf("ScheduleOfficialKeyword(infeasible) error = %v, want ErrKeywordChangeNotFeasible", err)
	}
	var active bool
	if err := pool.QueryRow(ctx, `SELECT is_active FROM official_keywords WHERE id = $1`, keywordID).Scan(&active); err != nil {
		t.Fatalf("query active after rejected deactivation: %v", err)
	}
	if !active {
		t.Fatal("keyword changed despite rejected deactivation")
	}
	var changeCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM official_keyword_changes`).Scan(&changeCount); err != nil {
		t.Fatalf("count changes after rejected deactivation: %v", err)
	}
	if changeCount != 0 {
		t.Fatalf("change count after rejected deactivation = %d, want 0", changeCount)
	}
}
func TestOfficialPublishSessionResamplesDateAfterRotationLock(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	oldNow := time.Date(2026, time.September, 14, 15, 59, 59, 0, time.UTC)
	oldToday := common.NormalizeBizDate(oldNow)
	clock := newTestClock(oldNow)
	catalog, _ := newTestCatalogService(pool, clock.Now)
	if err := catalog.InitializeRotation(ctx, oldToday); err != nil {
		t.Fatalf("InitializeRotation() error = %v", err)
	}
	ownerID := uuid.New()
	seedUser(t, ctx, pool, ownerID, "publish-lock-wait@example.com", "publish-lock-wait")
	svc := publishingapp.NewService(pool, catalog, clock.Now)

	lockTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin rotation lock tx: %v", err)
	}
	defer func() { _ = lockTx.Rollback(ctx) }()
	if _, err := lockTx.Exec(ctx, `SELECT 1 FROM official_keyword_rotation_state WHERE singleton FOR UPDATE`); err != nil {
		t.Fatalf("lock rotation state: %v", err)
	}

	resultCh := make(chan error, 1)
	go func() {
		_, err := svc.CreateOfficialSession(ctx, publishingapp.CreateOfficialSessionInput{
			OwnerUserID:       ownerID,
			OfficialKeywordID: uuid.New(),
			BizDate:           oldToday,
		})
		resultCh <- err
	}()

	waitForLockWait(t, ctx, pool)
	clock.Set(oldNow.Add(2 * time.Second))
	if err := lockTx.Commit(ctx); err != nil {
		t.Fatalf("commit rotation lock tx: %v", err)
	}
	select {
	case err := <-resultCh:
		if !errors.Is(err, common.ErrNotFound) {
			t.Fatalf("CreateOfficialSession(crossed midnight) error = %v, want not found", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("CreateOfficialSession did not finish after rotation lock release")
	}

	var assignmentCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM daily_keyword_assignments WHERE biz_date = $1`, oldToday).Scan(&assignmentCount); err != nil {
		t.Fatalf("count assignments after crossed midnight: %v", err)
	}
	if assignmentCount != 0 {
		t.Fatalf("crossed-midnight assignment count = %d, want 0", assignmentCount)
	}
}
func TestOfficialKeywordAdditionActivatesAtNextBusinessDay(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	effective := dateOnly(time.Date(2026, time.December, 28, 0, 0, 0, 0, time.UTC))
	now := effective.Add(12 * time.Hour)
	catalog, _ := newTestCatalogService(pool, func() time.Time { return now })
	if err := catalog.InitializeRotation(ctx, effective); err != nil {
		t.Fatalf("InitializeRotation() error = %v", err)
	}

	keywordID := uuid.New()
	order := int32(500)
	if _, err := catalog.ScheduleOfficialKeyword(ctx, dofficial.UpsertKeywordParams{
		ID:           keywordID,
		Text:         "next-day-keyword",
		Category:     dofficial.KeywordCategoryAbstract,
		IsActive:     true,
		DisplayOrder: &order,
	}); err != nil {
		t.Fatalf("ScheduleOfficialKeyword(add) error = %v", err)
	}
	var active bool
	if err := pool.QueryRow(ctx, `SELECT is_active FROM official_keywords WHERE id = $1`, keywordID).Scan(&active); err != nil {
		t.Fatalf("query new keyword before effective date: %v", err)
	}
	if active {
		t.Fatal("new keyword became active before its effective date")
	}

	now = effective.AddDate(0, 0, 1).Add(12 * time.Hour)
	if _, err := catalog.EnsureDailyKeywordAssignment(ctx, effective.AddDate(0, 0, 1)); err != nil {
		t.Fatalf("EnsureDailyKeywordAssignment(addition effective date) error = %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT is_active FROM official_keywords WHERE id = $1`, keywordID).Scan(&active); err != nil {
		t.Fatalf("query new keyword on effective date: %v", err)
	}
	if !active {
		t.Fatal("new keyword did not activate on its effective date")
	}
	var queueCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM official_keyword_rotation_queue WHERE keyword_id = $1`, keywordID).Scan(&queueCount); err != nil {
		t.Fatalf("query new keyword queue row: %v", err)
	}
	if queueCount != 1 {
		t.Fatalf("new keyword queue row count = %d, want 1", queueCount)
	}
}
func TestEnsureRotationInitializedPersistsNextBusinessDateOnce(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	now := time.Date(2026, time.December, 30, 10, 0, 0, 0, time.UTC)
	catalog, _ := newTestCatalogService(pool, func() time.Time { return now })
	if err := catalog.EnsureRotationInitialized(ctx); err != nil {
		t.Fatalf("EnsureRotationInitialized() error = %v", err)
	}
	var effectiveDate time.Time
	if err := pool.QueryRow(ctx, `SELECT effective_date FROM official_keyword_rotation_state WHERE singleton`).Scan(&effectiveDate); err != nil {
		t.Fatalf("query effective date: %v", err)
	}
	want := common.NormalizeBizDate(now).AddDate(0, 0, 1)
	if !common.NormalizeBizDate(effectiveDate).Equal(want) {
		t.Fatalf("effective date = %s, want %s", effectiveDate, want)
	}

	now = now.AddDate(0, 0, 5)
	if err := catalog.EnsureRotationInitialized(ctx); err != nil {
		t.Fatalf("EnsureRotationInitialized(repeat) error = %v", err)
	}
	var effectiveAfter time.Time
	if err := pool.QueryRow(ctx, `SELECT effective_date FROM official_keyword_rotation_state WHERE singleton`).Scan(&effectiveAfter); err != nil {
		t.Fatalf("query effective date after repeat: %v", err)
	}
	if !common.NormalizeBizDate(effectiveAfter).Equal(want) {
		t.Fatalf("effective date after repeat = %s, want unchanged %s", effectiveAfter, want)
	}
}
