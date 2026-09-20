package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	analyticsapp "cixing/internal/modules/analytics/application"
	analyticsdb "cixing/internal/modules/analytics/infra/db/gen"
	analyticsrepo "cixing/internal/modules/analytics/infra/db/repo"
	officialapp "cixing/internal/modules/official/application"
	officialdb "cixing/internal/modules/official/infra/db/gen"
	officialrepo "cixing/internal/modules/official/infra/db/repo"
	"cixing/internal/transport/http/server"
	v1 "cixing/internal/transport/http/v1"
	v1gen "cixing/internal/transport/http/v1/gen"
)

type testAccessTokenVerifier struct {
	users map[string]string
}

func (v testAccessTokenVerifier) VerifyAccessToken(_ context.Context, token string) (string, error) {
	if userID, ok := v.users[token]; ok {
		return userID, nil
	}
	return "", errors.New("invalid token")
}

type httpTestEnv struct {
	pool      *pgxpool.Pool
	clock     *testClock
	official  *officialapp.PromptService
	analytics *analyticsapp.Service
}

func newHTTPTestEnv(t *testing.T, pool *pgxpool.Pool, now time.Time) *httpTestEnv {
	t.Helper()
	clock := newTestClock(now)
	officialRepo := officialrepo.NewRepository(officialdb.New(pool))
	catalog := officialapp.NewCatalogService(pool, officialRepo, clock.Now)
	analyticsRepo := analyticsrepo.NewRepository(analyticsdb.New(pool))
	return &httpTestEnv{
		pool:      pool,
		clock:     clock,
		official:  officialapp.NewPromptService(pool, officialRepo, catalog, clock.Now),
		analytics: analyticsapp.NewService(pool, analyticsRepo, clock.Now),
	}
}

func (e *httpTestEnv) router(users map[string]string, rps, burst int) *gin.Engine {
	return server.NewRouter(server.Options{
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		GinMode: gin.TestMode,
		V1: &v1.Handler{
			OfficialPrompts: e.official,
			Analytics:       e.analytics,
		},
		AccessTokenVerifier: testAccessTokenVerifier{users: users},
		RateLimitRPS:        rps,
		RateLimitBurst:      burst,
	})
}

func performRequest(t *testing.T, router http.Handler, method, path, token string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

type responseError struct {
	Code      string `json:"code"`
	Reason    string `json:"reason"`
	RequestID string `json:"request_id"`
}

func decodeErrorResponse(t *testing.T, rec *httptest.ResponseRecorder) responseError {
	t.Helper()
	var payload responseError
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v; body=%s", err, rec.Body.String())
	}
	return payload
}

// Invalid requests are rejected before service calls, so this regression test
// exercises the real middleware chain without requiring a database.
func TestBodyLimitUsesActualOverflowNotValidationMessage(t *testing.T) {
	env := &httpTestEnv{}
	router := env.router(map[string]string{"token": uuid.NewString()}, 0, 0)
	analyticsPath := "/v1/analytics/events/batch"
	smallBody := []byte(`{"events":[],"analytics request body exceeds configured limit":true}`)
	tests := []struct {
		name          string
		path          string
		body          []byte
		unknownLength bool
		wantStatus    int
	}{
		{name: "small invalid analytics", path: analyticsPath, body: smallBody, wantStatus: http.StatusBadRequest},
		{name: "small unknown length", path: analyticsPath, body: smallBody, unknownLength: true, wantStatus: http.StatusBadRequest},
		{name: "other endpoint", path: "/v1/official/keywords/" + uuid.NewString() + "/prompts/draw", body: []byte(`{"kind":"analytics request body exceeds configured limit"}`), wantStatus: http.StatusBadRequest},
		{name: "exact limit", path: analyticsPath, body: padJSONToSize(t, smallBody, 32768), unknownLength: true, wantStatus: http.StatusBadRequest},
		{name: "known length overflow", path: analyticsPath, body: padJSONToSize(t, []byte(`{"events":[]}`), 32769), wantStatus: http.StatusRequestEntityTooLarge},
		{name: "unknown length overflow", path: analyticsPath, body: padJSONToSize(t, []byte(`{"events":[]}`), 32769), unknownLength: true, wantStatus: http.StatusRequestEntityTooLarge},
		{name: "next request after overflow", path: analyticsPath, body: smallBody, wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer token")
			req.Header.Set("Content-Type", "application/json")
			if tt.unknownLength {
				req.ContentLength = -1
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("%d-byte request: status=%d, want %d; body=%s", len(tt.body), rec.Code, tt.wantStatus, rec.Body.String())
			}
			payload := decodeErrorResponse(t, rec)
			if payload.Code != "validation" || payload.Reason != "validation.failed" || payload.RequestID == "" {
				t.Fatalf("unexpected error envelope: %+v", payload)
			}
		})
	}
}

func TestDailyPromptHTTPContractAndErrorMapping(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	now := time.Date(2026, time.September, 18, 2, 30, 0, 0, time.UTC)
	bizDate := dateOnly(now)
	userID := uuid.New()
	keywordID := uuid.New()
	seedUser(t, ctx, pool, userID, "daily-http@example.com", "daily-http")
	seedOfficialKeywordAsDaily(t, ctx, pool, keywordID, "HTTP关键词", bizDate)
	seedOfficialPrompt(t, ctx, pool, uuid.New(), keywordID, "structure", "HTTP 结构提示")

	env := newHTTPTestEnv(t, pool, now)
	router := env.router(map[string]string{"valid-token": userID.String()}, 0, 0)

	rec := performRequest(t, router, http.MethodGet, "/v1/me/daily-prompt", "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("GET without token status = %d, want 401; body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, router, http.MethodGet, "/v1/me/daily-prompt", "valid-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET unselected status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var daily v1gen.DailyPromptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &daily); err != nil {
		t.Fatalf("decode daily response: %v", err)
	}
	if daily.Selection != nil || daily.KeywordId == nil || uuid.UUID(*daily.KeywordId) != keywordID {
		t.Fatalf("GET unselected response = %+v", daily)
	}

	drawPath := "/v1/official/keywords/" + keywordID.String() + "/prompts/draw"
	rec = performRequest(t, router, http.MethodPost, drawPath, "valid-token", []byte(`{"kind":"structure","biz_date":null}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST explicit null biz_date status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, router, http.MethodPost, drawPath, "valid-token", []byte(`{"kind":"structure","biz_date":"2026-02-30"}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST invalid calendar date status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, router, http.MethodPost, drawPath, "valid-token", []byte(`{"kind":"structure","biz_date":"2026-09-17"}`))
	if rec.Code != http.StatusConflict {
		t.Fatalf("POST old biz_date status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeErrorResponse(t, rec); payload.Reason != "official.prompt_date_changed" {
		t.Fatalf("POST old biz_date reason = %q", payload.Reason)
	}

	mismatchPath := "/v1/official/keywords/" + uuid.NewString() + "/prompts/draw"
	rec = performRequest(t, router, http.MethodPost, mismatchPath, "valid-token", []byte(`{"kind":"structure","biz_date":"2026-09-18"}`))
	if rec.Code != http.StatusConflict {
		t.Fatalf("POST keyword mismatch status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeErrorResponse(t, rec); payload.Reason != "official.keyword_mismatch" {
		t.Fatalf("POST keyword mismatch reason = %q", payload.Reason)
	}

	rec = performRequest(t, router, http.MethodPost, drawPath, "valid-token", []byte(`{"kind":"concept","biz_date":"2026-09-18"}`))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("POST no candidate status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeErrorResponse(t, rec); payload.Reason != "official.prompt_not_available" {
		t.Fatalf("POST no candidate reason = %q", payload.Reason)
	}

	rec = performRequest(t, router, http.MethodPost, drawPath, "valid-token", []byte(`{"kind":"invalid","biz_date":"2026-09-18"}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST invalid kind status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, router, http.MethodPost, drawPath, "valid-token", []byte(`{"kind":"structure","biz_date":"2026-09-18"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST first draw status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var first v1gen.DrawPromptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode first draw: %v", err)
	}
	rec = performRequest(t, router, http.MethodPost, mismatchPath, "valid-token", []byte(`{"kind":"intuition"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("POST repeat different context status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var repeated v1gen.DrawPromptResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &repeated); err != nil {
		t.Fatalf("decode repeated draw: %v", err)
	}
	if repeated.Id != first.Id || repeated.Kind != first.Kind || repeated.Content != first.Content || !repeated.SelectedAt.Equal(first.SelectedAt) {
		t.Fatalf("repeated draw = %+v, want %+v", repeated, first)
	}

	rec = performRequest(t, router, http.MethodGet, "/v1/me/daily-prompt", "valid-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET selected status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &daily); err != nil {
		t.Fatalf("decode selected daily response: %v", err)
	}
	if daily.Selection == nil || uuid.UUID(daily.Selection.Id) != uuid.UUID(first.Id) || daily.Selection.Content != first.Content {
		t.Fatalf("GET selected response = %+v", daily)
	}

	rateRouter := env.router(map[string]string{"valid-token": userID.String()}, 1, 1)
	if rec = performRequest(t, rateRouter, http.MethodGet, "/v1/me/daily-prompt", "valid-token", nil); rec.Code != http.StatusOK {
		t.Fatalf("rate-limit first request status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if rec = performRequest(t, rateRouter, http.MethodGet, "/v1/me/daily-prompt", "valid-token", nil); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("rate-limit second request status = %d, want 429; body=%s", rec.Code, rec.Body.String())
	}

	if _, err := pool.Exec(ctx, `DROP TABLE user_daily_prompts`); err != nil {
		t.Fatalf("drop user_daily_prompts for 500 test: %v", err)
	}
	rec = performRequest(t, router, http.MethodGet, "/v1/me/daily-prompt", "valid-token", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("GET storage failure status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyticsHTTPContractDeduplicationAndBodyLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	now := time.Date(2026, time.September, 18, 2, 30, 0, 0, time.UTC)
	user1 := uuid.New()
	user2 := uuid.New()
	seedUser(t, ctx, pool, user1, "analytics-http-one@example.com", "analytics-http-one")
	seedUser(t, ctx, pool, user2, "analytics-http-two@example.com", "analytics-http-two")

	env := newHTTPTestEnv(t, pool, now)
	users := map[string]string{"token-1": user1.String(), "token-2": user2.String()}
	router := env.router(users, 0, 0)

	rec := performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-1", analyticsHTTPBody(t, uuid.New(), "prompt_entry_click", nil, "unknown"))
	if rec.Code != http.StatusOK {
		t.Fatalf("valid analytics batch status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var ack v1gen.AnalyticsEventsBatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &ack); err != nil {
		t.Fatalf("decode analytics response: %v", err)
	}
	if len(ack.AcknowledgedEventIds) != 1 {
		t.Fatalf("acknowledged ids = %v", ack.AcknowledgedEventIds)
	}
	rec = performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-1", analyticsHTTPBody(t, uuid.New(), "prompt_kind_entry_click", strPtr("structure"), "unselected"))
	if rec.Code != http.StatusOK {
		t.Fatalf("valid kind-entry batch status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-1", analyticsHTTPBody(t, uuid.New(), "prompt_entry_click", strPtr("structure"), "unknown"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid event combination status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	extraField := []byte(`{"events":[{"event_id":"` + uuid.NewString() + `","event_name":"prompt_entry_click","kind":null,"source":"today","schema_version":1,"occurred_at":"2026-09-18T10:00:00+08:00","selection_state":"unknown","user_id":"` + user1.String() + `"}]}`)
	rec = performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-1", extraField)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("extra analytics field status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	invalidDateEvent := analyticsHTTPEvent(uuid.New(), "prompt_entry_click", nil, "unknown")
	invalidDateEvent["context_biz_date"] = "2026-02-30"
	invalidDateBody, err := json.Marshal(map[string]any{"events": []map[string]any{invalidDateEvent}})
	if err != nil {
		t.Fatalf("marshal invalid context date: %v", err)
	}
	rec = performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-1", invalidDateBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid context date status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	noTimezoneEvent := analyticsHTTPEvent(uuid.New(), "prompt_entry_click", nil, "unknown")
	noTimezoneEvent["occurred_at"] = "2026-09-18T10:00:00"
	noTimezoneBody, err := json.Marshal(map[string]any{"events": []map[string]any{noTimezoneEvent}})
	if err != nil {
		t.Fatalf("marshal timezone-less occurred_at: %v", err)
	}
	rec = performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-1", noTimezoneBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("timezone-less occurred_at status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	tooMany := make([]map[string]any, 21)
	for i := range tooMany {
		tooMany[i] = analyticsHTTPEvent(uuid.New(), "prompt_entry_click", nil, "unknown")
	}
	tooManyBody, err := json.Marshal(map[string]any{"events": tooMany})
	if err != nil {
		t.Fatalf("marshal 21 events: %v", err)
	}
	rec = performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-1", tooManyBody)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("21 events status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	rec = performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-1", []byte(`{"events":[]}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("0 events status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}

	twenty := make([]map[string]any, 20)
	for i := range twenty {
		twenty[i] = analyticsHTTPEvent(uuid.New(), "prompt_entry_click", nil, "unknown")
	}
	twentyBody, err := json.Marshal(map[string]any{"events": twenty})
	if err != nil {
		t.Fatalf("marshal 20 events: %v", err)
	}
	rec = performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-1", twentyBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("20 events status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	exactBody := padJSONToSize(t, analyticsHTTPBody(t, uuid.New(), "prompt_entry_click", nil, "unknown"), 32768)
	rec = performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-1", exactBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("32768-byte analytics body status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	overBody := padJSONToSize(t, analyticsHTTPBody(t, uuid.New(), "prompt_entry_click", nil, "unknown"), 32769)
	rec = performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-1", overBody)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("32769-byte analytics body status = %d, want 413; body=%s", rec.Code, rec.Body.String())
	}
	if payload := decodeErrorResponse(t, rec); payload.Code != "validation" || payload.Reason != "validation.failed" {
		t.Fatalf("413 payload = %+v", payload)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/analytics/events/batch", bytes.NewReader(overBody))
	req.Header.Set("Authorization", "Bearer token-1")
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = -1
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("unknown-length oversized body status = %d, want 413; body=%s", rec.Code, rec.Body.String())
	}

	sharedID := uuid.New()
	if rec = performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-1", analyticsHTTPBody(t, sharedID, "prompt_entry_click", nil, "unknown")); rec.Code != http.StatusOK {
		t.Fatalf("user1 shared id status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if rec = performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-2", analyticsHTTPBody(t, sharedID, "prompt_entry_click", nil, "unknown")); rec.Code != http.StatusOK {
		t.Fatalf("user2 shared id status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var sharedCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM analytics_events WHERE event_id = $1`, sharedID).Scan(&sharedCount); err != nil {
		t.Fatalf("count shared event id: %v", err)
	}
	if sharedCount != 2 {
		t.Fatalf("shared event id count = %d, want 2", sharedCount)
	}

	rateRouter := env.router(users, 1, 1)
	if rec = performRequest(t, rateRouter, http.MethodPost, "/v1/analytics/events/batch", "token-1", analyticsHTTPBody(t, uuid.New(), "prompt_entry_click", nil, "unknown")); rec.Code != http.StatusOK {
		t.Fatalf("analytics rate-limit first status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if rec = performRequest(t, rateRouter, http.MethodPost, "/v1/analytics/events/batch", "token-1", analyticsHTTPBody(t, uuid.New(), "prompt_entry_click", nil, "unknown")); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("analytics rate-limit second status = %d, want 429; body=%s", rec.Code, rec.Body.String())
	}

	if _, err := pool.Exec(ctx, `DROP TABLE analytics_events`); err != nil {
		t.Fatalf("drop analytics_events for 500 test: %v", err)
	}
	rec = performRequest(t, router, http.MethodPost, "/v1/analytics/events/batch", "token-1", analyticsHTTPBody(t, uuid.New(), "prompt_entry_click", nil, "unknown"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("analytics storage failure status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
}

func analyticsHTTPBody(t *testing.T, eventID uuid.UUID, eventName string, kind *string, selectionState string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{"events": []map[string]any{
		analyticsHTTPEvent(eventID, eventName, kind, selectionState),
	}})
	if err != nil {
		t.Fatalf("marshal analytics body: %v", err)
	}
	return body
}

func analyticsHTTPEvent(eventID uuid.UUID, eventName string, kind *string, selectionState string) map[string]any {
	return map[string]any{
		"event_id":         eventID.String(),
		"event_name":       eventName,
		"kind":             kind,
		"source":           "today",
		"schema_version":   1,
		"occurred_at":      "2026-09-18T10:00:00+08:00",
		"context_biz_date": "2026-09-18",
		"keyword_id":       nil,
		"selection_state":  selectionState,
	}
}

func padJSONToSize(t *testing.T, body []byte, size int) []byte {
	t.Helper()
	if len(body) > size {
		t.Fatalf("body length %d exceeds target %d", len(body), size)
	}
	return append(body, bytes.Repeat([]byte(" "), size-len(body))...)
}
