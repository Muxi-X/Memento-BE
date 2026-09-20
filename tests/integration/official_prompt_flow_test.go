package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	officialapp "cixing/internal/modules/official/application"
	officialdb "cixing/internal/modules/official/infra/db/gen"
	officialrepo "cixing/internal/modules/official/infra/db/repo"
	"cixing/internal/shared/common"
)

type testClock struct {
	mu  sync.RWMutex
	now time.Time
}

func newTestClock(now time.Time) *testClock {
	return &testClock{now: now}
}

func (c *testClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *testClock) Set(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = now
}

func newOfficialPromptService(pool *pgxpool.Pool, clock *testClock) (*officialapp.PromptService, *officialrepo.Repository) {
	repo := officialrepo.NewRepository(officialdb.New(pool))
	catalog := officialapp.NewCatalogService(pool, repo, clock.Now)
	return officialapp.NewPromptService(pool, repo, catalog, clock.Now), repo
}

func seedOfficialPrompt(t *testing.T, ctx context.Context, pool *pgxpool.Pool, promptID, keywordID uuid.UUID, kind, content string) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO official_keyword_prompts (id, keyword_id, kind, content, display_order, is_active)
		VALUES ($1, $2, $3, $4, 1, TRUE)
	`, promptID, keywordID, kind, content); err != nil {
		t.Fatalf("insert official_keyword_prompts: %v", err)
	}
}

func dailyPromptCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, bizDate time.Time) int {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM user_daily_prompts WHERE user_id = $1 AND biz_date = $2`, userID, dateOnly(bizDate)).Scan(&count); err != nil {
		t.Fatalf("count user_daily_prompts: %v", err)
	}
	return count
}

func TestOfficialDailyPromptLifecycleAndSnapshot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	now := time.Date(2026, time.September, 18, 2, 30, 0, 0, time.UTC) // 10:30 Asia/Shanghai
	bizDate := common.NormalizeBizDate(now)
	clock := newTestClock(now)

	userID := uuid.MustParse("61111111-1111-4111-8111-111111111111")
	keywordID := uuid.MustParse("62222222-2222-4222-8222-222222222222")
	structurePromptID := uuid.MustParse("63333333-3333-4333-8333-333333333333")
	seedUser(t, ctx, pool, userID, "daily-lifecycle@example.com", "daily-lifecycle")
	seedOfficialKeywordAsDaily(t, ctx, pool, keywordID, "每日生命周期", bizDate)
	seedOfficialPrompt(t, ctx, pool, structurePromptID, keywordID, "structure", "你和别人之间，隔着多远？")
	seedOfficialPrompt(t, ctx, pool, uuid.New(), keywordID, "intuition", "你最先注意到什么？")

	service, _ := newOfficialPromptService(pool, clock)
	state, err := service.GetDailyPrompt(ctx, userID)
	if err != nil {
		t.Fatalf("GetDailyPrompt() error = %v", err)
	}
	if state.Selection != nil || state.KeywordID == nil || *state.KeywordID != keywordID {
		t.Fatalf("initial daily state = %+v, want unselected keyword %s", state, keywordID)
	}
	if !state.BizDate.Equal(bizDate) {
		t.Fatalf("initial biz_date = %s, want %s", state.BizDate, bizDate)
	}

	first, err := service.Draw(ctx, userID, keywordID, "structure", &bizDate)
	if err != nil {
		t.Fatalf("Draw(first) error = %v", err)
	}
	if first.ID != structurePromptID || first.Kind != "structure" || first.Content != "你和别人之间，隔着多远？" {
		t.Fatalf("first draw = %+v", first)
	}
	if first.SelectedAt.IsZero() {
		t.Fatal("first draw selected_at is zero")
	}
	wantReset := time.Date(2026, time.September, 19, 0, 0, 0, 0, common.BusinessLocation())
	if !first.ResetsAt.Equal(wantReset) {
		t.Fatalf("first draw resets_at = %s, want %s", first.ResetsAt, wantReset)
	}

	state, err = service.GetDailyPrompt(ctx, userID)
	if err != nil {
		t.Fatalf("GetDailyPrompt(selected) error = %v", err)
	}
	if state.Selection == nil || state.Selection.ID != structurePromptID || state.KeywordID == nil || *state.KeywordID != keywordID {
		t.Fatalf("selected daily state = %+v", state)
	}

	repeated, err := service.Draw(ctx, userID, uuid.New(), "intuition", &bizDate)
	if err != nil {
		t.Fatalf("Draw(repeated different context) error = %v", err)
	}
	if repeated.ID != first.ID || repeated.Kind != first.Kind || repeated.Content != first.Content || !repeated.SelectedAt.Equal(first.SelectedAt) {
		t.Fatalf("repeated draw = %+v, want original %+v", repeated, first)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM official_keyword_prompts WHERE id = $1`, structurePromptID); err != nil {
		t.Fatalf("delete source prompt: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE official_keywords SET is_active = FALSE WHERE id = $1`, keywordID); err != nil {
		t.Fatalf("deactivate source keyword: %v", err)
	}

	afterSourceChange, err := service.Draw(ctx, userID, uuid.New(), "concept", &bizDate)
	if err != nil {
		t.Fatalf("Draw(after source change) error = %v", err)
	}
	if afterSourceChange.ID != first.ID || afterSourceChange.Content != first.Content || !afterSourceChange.SelectedAt.Equal(first.SelectedAt) {
		t.Fatalf("draw after source change = %+v, want %+v", afterSourceChange, first)
	}
	state, err = service.GetDailyPrompt(ctx, userID)
	if err != nil {
		t.Fatalf("GetDailyPrompt(after source change) error = %v", err)
	}
	if state.Selection == nil || state.Selection.ID != first.ID || state.Selection.Content != first.Content {
		t.Fatalf("state after source change = %+v", state)
	}

	wrongDate := bizDate.AddDate(0, 0, -1)
	if _, err := service.Draw(ctx, userID, keywordID, "structure", &wrongDate); !errors.Is(err, officialapp.ErrPromptDateChanged) {
		t.Fatalf("Draw(wrong date) error = %v, want ErrPromptDateChanged", err)
	}
	if _, err := service.Draw(ctx, userID, keywordID, "invalid", &bizDate); !errors.Is(err, officialapp.ErrInvalidPromptKind) {
		t.Fatalf("Draw(invalid kind) error = %v, want ErrInvalidPromptKind", err)
	}
	if got := dailyPromptCount(t, ctx, pool, userID, bizDate); got != 1 {
		t.Fatalf("daily prompt count = %d, want 1", got)
	}
}

func TestOfficialDailyPromptFirstClaimValidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	now := time.Date(2026, time.September, 18, 2, 30, 0, 0, time.UTC)
	bizDate := common.NormalizeBizDate(now)
	clock := newTestClock(now)
	service, _ := newOfficialPromptService(pool, clock)

	keywordA := uuid.New()
	keywordB := uuid.New()
	userMismatch := uuid.New()
	seedUser(t, ctx, pool, userMismatch, "daily-mismatch@example.com", "daily-mismatch")
	seedOfficialKeywordAsDaily(t, ctx, pool, keywordA, "关键词A", bizDate)
	seedOfficialPrompt(t, ctx, pool, uuid.New(), keywordA, "structure", "A 的结构提示")
	if _, err := service.Draw(ctx, userMismatch, keywordB, "structure", &bizDate); !errors.Is(err, officialapp.ErrKeywordMismatch) {
		t.Fatalf("Draw(keyword mismatch) error = %v, want ErrKeywordMismatch", err)
	}
	if got := dailyPromptCount(t, ctx, pool, userMismatch, bizDate); got != 0 {
		t.Fatalf("mismatch daily prompt count = %d, want 0", got)
	}

	keywordC := uuid.New()
	userNoCandidate := uuid.New()
	seedUser(t, ctx, pool, userNoCandidate, "daily-no-candidate@example.com", "daily-no-candidate")
	seedOfficialKeywordAsDaily(t, ctx, pool, keywordC, "关键词C", bizDate)
	if _, err := service.Draw(ctx, userNoCandidate, keywordC, "concept", &bizDate); !errors.Is(err, officialapp.ErrPromptNotAvailable) {
		t.Fatalf("Draw(no candidate) error = %v, want ErrPromptNotAvailable", err)
	}
	if got := dailyPromptCount(t, ctx, pool, userNoCandidate, bizDate); got != 0 {
		t.Fatalf("no-candidate daily prompt count = %d, want 0", got)
	}
}

func TestOfficialDailyPromptConcurrentDrawReturnsPersistedWinner(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	now := time.Date(2026, time.September, 18, 2, 30, 0, 0, time.UTC)
	bizDate := common.NormalizeBizDate(now)
	clock := newTestClock(now)
	service, _ := newOfficialPromptService(pool, clock)

	userID := uuid.New()
	keywordID := uuid.New()
	seedUser(t, ctx, pool, userID, "daily-concurrent@example.com", "daily-concurrent")
	seedOfficialKeywordAsDaily(t, ctx, pool, keywordID, "并发关键词", bizDate)
	seedOfficialPrompt(t, ctx, pool, uuid.New(), keywordID, "structure", "并发的空间提示")
	seedOfficialPrompt(t, ctx, pool, uuid.New(), keywordID, "intuition", "并发的直觉提示")

	type result struct {
		out *officialapp.PromptOutput
		err error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, kind := range []string{"structure", "intuition"} {
		kind := kind
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			out, err := service.Draw(ctx, userID, keywordID, kind, &bizDate)
			results <- result{out: out, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var outputs []*officialapp.PromptOutput
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent Draw() error = %v", result.err)
		}
		outputs = append(outputs, result.out)
	}
	if len(outputs) != 2 {
		t.Fatalf("concurrent output count = %d, want 2", len(outputs))
	}

	var storedID uuid.UUID
	var storedKind, storedContent string
	if err := pool.QueryRow(ctx, `
		SELECT prompt_id, kind::text, content_snapshot
		FROM user_daily_prompts
		WHERE user_id = $1 AND biz_date = $2
	`, userID, dateOnly(bizDate)).Scan(&storedID, &storedKind, &storedContent); err != nil {
		t.Fatalf("query persisted winner: %v", err)
	}
	for _, out := range outputs {
		if out.ID != storedID || string(out.Kind) != storedKind || out.Content != storedContent {
			t.Fatalf("concurrent output = %+v, persisted = (%s,%s,%s)", out, storedID, storedKind, storedContent)
		}
	}
	if got := dailyPromptCount(t, ctx, pool, userID, bizDate); got != 1 {
		t.Fatalf("concurrent daily prompt count = %d, want 1", got)
	}
}

func TestOfficialDailyPromptSamplesClockAfterUserLock(t *testing.T) {
	ctx := context.Background()
	pool := openIntegrationDB(t)
	oldNow := time.Date(2026, time.September, 18, 15, 59, 59, 0, time.UTC) // 23:59:59 Asia/Shanghai
	oldBizDate := common.NormalizeBizDate(oldNow)
	clock := newTestClock(oldNow)

	userID := uuid.New()
	keywordID := uuid.New()
	seedUser(t, ctx, pool, userID, "daily-lock-date@example.com", "daily-lock-date")
	seedOfficialKeywordAsDaily(t, ctx, pool, keywordID, "跨日关键词", oldBizDate)
	seedOfficialPrompt(t, ctx, pool, uuid.New(), keywordID, "structure", "跨日提示")

	lockTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin lock tx: %v", err)
	}
	if _, err := lockTx.Exec(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, userID); err != nil {
		_ = lockTx.Rollback(ctx)
		t.Fatalf("lock user: %v", err)
	}

	clockEntered := make(chan struct{}, 1)
	releaseClock := make(chan struct{})
	service, _ := newOfficialPromptServiceWithClock(pool, func() time.Time {
		clockEntered <- struct{}{}
		<-releaseClock
		return clock.Now()
	})

	type result struct {
		out *officialapp.PromptOutput
		err error
	}
	resultCh := make(chan result, 1)
	go func() {
		out, err := service.Draw(ctx, userID, keywordID, "structure", &oldBizDate)
		resultCh <- result{out: out, err: err}
	}()

	waitForLockWait(t, ctx, pool)
	select {
	case <-clockEntered:
		t.Fatal("service sampled clock before acquiring the user lock")
	default:
	}

	clock.Set(oldNow.Add(2 * time.Second)) // 00:00:01 Asia/Shanghai next day
	if err := lockTx.Commit(ctx); err != nil {
		t.Fatalf("commit lock tx: %v", err)
	}

	select {
	case <-clockEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("service did not sample clock after acquiring the user lock")
	}
	close(releaseClock)

	select {
	case result := <-resultCh:
		if !errors.Is(result.err, officialapp.ErrPromptDateChanged) {
			t.Fatalf("Draw(lock crossed midnight) error = %v, want ErrPromptDateChanged", result.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Draw did not finish after lock release")
	}
	if got := dailyPromptCount(t, ctx, pool, userID, oldBizDate); got != 0 {
		t.Fatalf("daily prompt count on old date = %d, want 0", got)
	}
}

func newOfficialPromptServiceWithClock(pool *pgxpool.Pool, now func() time.Time) (*officialapp.PromptService, *officialrepo.Repository) {
	repo := officialrepo.NewRepository(officialdb.New(pool))
	catalog := officialapp.NewCatalogService(pool, repo, now)
	return officialapp.NewPromptService(pool, repo, catalog, now), repo
}

func waitForLockWait(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		if err := pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM pg_stat_activity
				WHERE datname = current_database()
				  AND wait_event_type = 'Lock'
			)
		`).Scan(&waiting); err != nil {
			t.Fatalf("query lock waiters: %v", err)
		}
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timeout waiting for a database lock waiter")
}

func TestOfficialPromptDrawRejectsInactiveKeyword(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)

	keywordID := uuid.MustParse("52222222-2222-2222-2222-222222222222")
	promptID := uuid.MustParse("53333333-3333-3333-3333-333333333333")
	userID := uuid.MustParse("51111111-1111-4111-8111-111111111111")
	seedUser(t, ctx, pool, userID, "inactive-prompt@example.com", "inactive-prompt")

	if _, err := pool.Exec(ctx, `
		INSERT INTO official_keywords (id, text, category, is_active, display_order)
		VALUES ($1, $2, 'emotion', FALSE, 1)
	`, keywordID, "停用关键词"); err != nil {
		t.Fatalf("insert official_keywords: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO official_keyword_prompts (id, keyword_id, kind, content, display_order, is_active)
		VALUES ($1, $2, 'intuition', $3, 1, TRUE)
	`, promptID, keywordID, "still active prompt"); err != nil {
		t.Fatalf("insert official_keyword_prompts: %v", err)
	}

	now := time.Date(2026, time.September, 18, 10, 0, 0, 0, time.UTC)
	seedDailyKeywordAssignment(t, ctx, pool, keywordID, now)
	clock := newTestClock(now)
	promptSvc, _ := newOfficialPromptService(pool, clock)

	bizDate := now
	_, err := promptSvc.Draw(ctx, userID, keywordID, "intuition", &bizDate)
	if !errors.Is(err, officialapp.ErrPromptNotAvailable) {
		t.Fatalf("Draw() error = %v, want prompt not available", err)
	}
}
