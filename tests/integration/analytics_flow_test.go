package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	analyticsapp "cixing/internal/modules/analytics/application"
	danalytics "cixing/internal/modules/analytics/domain"
	analyticsdb "cixing/internal/modules/analytics/infra/db/gen"
	analyticsrepo "cixing/internal/modules/analytics/infra/db/repo"
	"cixing/internal/shared/common"
)

func analyticsEvent(id uuid.UUID, name danalytics.EventName, kind *string, state danalytics.SelectionState, occurredAt time.Time) danalytics.Event {
	return danalytics.Event{
		EventID:        id,
		EventName:      name,
		Kind:           kind,
		Source:         "today",
		SchemaVersion:  1,
		OccurredAt:     occurredAt,
		SelectionState: state,
	}
}

func TestAnalyticsBatchDeduplicatesAndAggregatesByReceiptDate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	receivedAt := time.Date(2026, time.September, 18, 2, 30, 0, 0, time.UTC)
	clock := newTestClock(receivedAt)
	repo := analyticsrepo.NewRepository(analyticsdb.New(pool))
	service := analyticsapp.NewService(pool, repo, clock.Now)

	user1 := uuid.New()
	user2 := uuid.New()
	seedUser(t, ctx, pool, user1, "analytics-one@example.com", "analytics-one")
	seedUser(t, ctx, pool, user2, "analytics-two@example.com", "analytics-two")

	mainID := uuid.New()
	structureID := uuid.New()
	user2MainID := uuid.New()
	conceptID := uuid.New()
	structure := "structure"
	concept := "concept"
	occurredAt := receivedAt.Add(-2 * time.Hour)

	events := []danalytics.Event{
		analyticsEvent(mainID, danalytics.EventNamePromptEntryClick, nil, danalytics.SelectionStateSelected, occurredAt),
		analyticsEvent(structureID, danalytics.EventNamePromptKindEntryClick, &structure, danalytics.SelectionStateUnselected, occurredAt),
		analyticsEvent(user2MainID, danalytics.EventNamePromptEntryClick, nil, danalytics.SelectionStateUnknown, occurredAt),
		analyticsEvent(conceptID, danalytics.EventNamePromptKindEntryClick, &concept, danalytics.SelectionStateUnselected, occurredAt.Add(time.Minute)),
	}
	acknowledged, err := service.RecordBatch(ctx, user1, "req-analytics-1", events[:2])
	if err != nil {
		t.Fatalf("RecordBatch(user1) error = %v", err)
	}
	if len(acknowledged) != 2 || acknowledged[0] != mainID || acknowledged[1] != structureID {
		t.Fatalf("acknowledged = %v", acknowledged)
	}
	if _, err := service.RecordBatch(ctx, user2, "req-analytics-2", events[2:]); err != nil {
		t.Fatalf("RecordBatch(user2) error = %v", err)
	}

	var beforeReceived time.Time
	var beforeBizDate time.Time
	if err := pool.QueryRow(ctx, `
		SELECT received_at, biz_date
		FROM analytics_events
		WHERE user_id = $1 AND event_id = $2
	`, user1, mainID).Scan(&beforeReceived, &beforeBizDate); err != nil {
		t.Fatalf("query first event: %v", err)
	}

	retry := analyticsEvent(mainID, danalytics.EventNamePromptKindEntryClick, &structure, danalytics.SelectionStateUnselected, occurredAt.Add(24*time.Hour))
	acknowledged, err = service.RecordBatch(ctx, user1, "req-analytics-retry", []danalytics.Event{retry})
	if err != nil {
		t.Fatalf("RecordBatch(retry) error = %v", err)
	}
	if len(acknowledged) != 1 || acknowledged[0] != mainID {
		t.Fatalf("retry acknowledged = %v", acknowledged)
	}

	var afterReceived time.Time
	var afterBizDate time.Time
	var storedName, storedKind string
	if err := pool.QueryRow(ctx, `
		SELECT received_at, biz_date, event_name, COALESCE(kind::text, '')
		FROM analytics_events
		WHERE user_id = $1 AND event_id = $2
	`, user1, mainID).Scan(&afterReceived, &afterBizDate, &storedName, &storedKind); err != nil {
		t.Fatalf("query retried event: %v", err)
	}
	if !afterReceived.Equal(beforeReceived) || !afterBizDate.Equal(beforeBizDate) {
		t.Fatalf("retry updated receipt metadata: before=(%s,%s) after=(%s,%s)", beforeReceived, beforeBizDate, afterReceived, afterBizDate)
	}
	if storedName != string(danalytics.EventNamePromptEntryClick) || storedKind != "" {
		t.Fatalf("retry changed stored payload to (%s,%s)", storedName, storedKind)
	}

	if _, err := service.RecordBatch(ctx, user2, "req-analytics-cross-user", []danalytics.Event{events[0]}); err != nil {
		t.Fatalf("RecordBatch(cross-user same id) error = %v", err)
	}

	metrics, err := service.ClickMetrics(ctx, common.NormalizeBizDate(receivedAt))
	if err != nil {
		t.Fatalf("ClickMetrics() error = %v", err)
	}
	if len(metrics) != 4 {
		t.Fatalf("ClickMetrics() rows = %d, want 4: %+v", len(metrics), metrics)
	}
	metricByKey := make(map[string]danalytics.ClickMetric, len(metrics))
	for _, metric := range metrics {
		metricByKey[string(metric.EventName)+":"+metric.Kind] = metric
	}
	assertMetric := func(eventName, kind string, clicks, users int64) {
		t.Helper()
		metric := metricByKey[eventName+":"+kind]
		if metric.ClickCount != clicks || metric.UniqueUserCount != users {
			t.Fatalf("metric %s/%s = (%d,%d), want (%d,%d)", eventName, kind, metric.ClickCount, metric.UniqueUserCount, clicks, users)
		}
	}
	assertMetric("prompt_entry_click", "", 3, 2)
	assertMetric("prompt_kind_entry_click", "intuition", 0, 0)
	assertMetric("prompt_kind_entry_click", "structure", 1, 1)
	assertMetric("prompt_kind_entry_click", "concept", 1, 1)

	insertDailyPromptForTest(t, ctx, pool, user1, receivedAt, uuid.New(), "structure")
	insertDailyPromptForTest(t, ctx, pool, user2, receivedAt, uuid.New(), "concept")
	successCount, err := service.DailyPromptSuccessCount(ctx, common.NormalizeBizDate(receivedAt))
	if err != nil {
		t.Fatalf("DailyPromptSuccessCount() error = %v", err)
	}
	if successCount != 2 {
		t.Fatalf("DailyPromptSuccessCount() = %d, want 2", successCount)
	}

	clock.Set(receivedAt.Add(24 * time.Hour))
	if _, err := service.RecordBatch(ctx, user1, "req-analytics-next-day-retry", []danalytics.Event{retry}); err != nil {
		t.Fatalf("RecordBatch(next day retry) error = %v", err)
	}
	nextDayMetrics, err := service.ClickMetrics(ctx, common.NormalizeBizDate(clock.Now()))
	if err != nil {
		t.Fatalf("ClickMetrics(next day) error = %v", err)
	}
	for _, metric := range nextDayMetrics {
		if metric.ClickCount != 0 || metric.UniqueUserCount != 0 {
			t.Fatalf("next-day metric = %+v, want zero", metric)
		}
	}
}

func TestAnalyticsBatchRollsBackAllNewEventsOnStorageFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	now := time.Date(2026, time.September, 18, 2, 30, 0, 0, time.UTC)
	clock := newTestClock(now)
	repo := analyticsrepo.NewRepository(analyticsdb.New(pool))
	service := analyticsapp.NewService(pool, repo, clock.Now)

	userID := uuid.New()
	seedUser(t, ctx, pool, userID, "analytics-rollback@example.com", "analytics-rollback")
	validID := uuid.New()
	failingID := uuid.New()
	if _, err := pool.Exec(ctx, `
		CREATE FUNCTION reject_analytics_event() RETURNS trigger AS $$
		BEGIN
			IF NEW.event_id = '`+failingID.String()+`'::uuid THEN
				RAISE EXCEPTION 'forced analytics insert failure';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_analytics_event_trigger
		BEFORE INSERT ON analytics_events
		FOR EACH ROW EXECUTE FUNCTION reject_analytics_event();
	`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	events := []danalytics.Event{
		analyticsEvent(validID, danalytics.EventNamePromptEntryClick, nil, danalytics.SelectionStateUnknown, now),
		analyticsEvent(failingID, danalytics.EventNamePromptEntryClick, nil, danalytics.SelectionStateUnknown, now),
	}
	if _, err := service.RecordBatch(ctx, userID, "req-analytics-rollback", events); err == nil {
		t.Fatal("RecordBatch() error = nil, want storage failure")
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM analytics_events WHERE user_id = $1`, userID).Scan(&count); err != nil {
		t.Fatalf("count analytics events: %v", err)
	}
	if count != 0 {
		t.Fatalf("analytics event count = %d, want 0 after rollback", count)
	}
}

func TestAnalyticsConcurrentDuplicateEventIsStoredOnce(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	now := time.Date(2026, time.September, 18, 2, 30, 0, 0, time.UTC)
	clock := newTestClock(now)
	repo := analyticsrepo.NewRepository(analyticsdb.New(pool))
	service := analyticsapp.NewService(pool, repo, clock.Now)

	userID := uuid.New()
	seedUser(t, ctx, pool, userID, "analytics-concurrent@example.com", "analytics-concurrent")
	event := analyticsEvent(uuid.New(), danalytics.EventNamePromptEntryClick, nil, danalytics.SelectionStateUnknown, now)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := service.RecordBatch(ctx, userID, "req-analytics-concurrent", []danalytics.Event{event})
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent RecordBatch() error = %v", err)
		}
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM analytics_events WHERE user_id = $1 AND event_id = $2`, userID, event.EventID).Scan(&count); err != nil {
		t.Fatalf("count concurrent event: %v", err)
	}
	if count != 1 {
		t.Fatalf("concurrent event count = %d, want 1", count)
	}
}

func TestAnalyticsStorageFailureDoesNotAffectDailyPromptClaim(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	now := time.Date(2026, time.September, 18, 2, 30, 0, 0, time.UTC)
	bizDate := dateOnly(now)
	userID := uuid.New()
	keywordID := uuid.New()
	seedUser(t, ctx, pool, userID, "analytics-independent@example.com", "analytics-independent")
	seedOfficialKeywordAsDaily(t, ctx, pool, keywordID, "独立关键词", bizDate)
	seedOfficialPrompt(t, ctx, pool, uuid.New(), keywordID, "structure", "独立提示")

	if _, err := pool.Exec(ctx, `DROP TABLE analytics_events`); err != nil {
		t.Fatalf("drop analytics_events: %v", err)
	}
	clock := newTestClock(now)
	promptSvc, _ := newOfficialPromptService(pool, clock)
	out, err := promptSvc.Draw(ctx, userID, keywordID, "structure", &now)
	if err != nil {
		t.Fatalf("Draw() with analytics table missing error = %v", err)
	}
	if out == nil || out.Content != "独立提示" {
		t.Fatalf("Draw() with analytics table missing = %+v", out)
	}
}

func insertDailyPromptForTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, selectedAt time.Time, promptID uuid.UUID, kind string) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO user_daily_prompts (
			user_id, biz_date, keyword_id, prompt_id, kind, content_snapshot, selected_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, userID, dateOnly(selectedAt), uuid.New(), promptID, kind, "统计提示内容", selectedAt); err != nil {
		t.Fatalf("insert user_daily_prompt: %v", err)
	}
}
