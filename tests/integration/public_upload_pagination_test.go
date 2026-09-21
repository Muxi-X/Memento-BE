package integration

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	readmodelapp "cixing/internal/modules/readmodel/application"
	readmodeldb "cixing/internal/modules/readmodel/infra/db/gen"
	readmodelrepo "cixing/internal/modules/readmodel/infra/db/repo"
	platformoss "cixing/internal/platform/oss"
	"cixing/internal/transport/http/server"
	v1 "cixing/internal/transport/http/v1"
)

func pagePtr[T any](v T) *T { return &v }

type publicPageFixture struct {
	pool          *pgxpool.Pool
	svc           *readmodelapp.Service
	user, keyword uuid.UUID
	day           time.Time
}

func newPublicPageFixture(t *testing.T, pool *pgxpool.Pool, day time.Time) *publicPageFixture {
	t.Helper()
	f := &publicPageFixture{pool: pool, user: uuid.New(), keyword: uuid.New(), day: day}
	seedUser(t, context.Background(), pool, f.user, f.user.String()+"@pagination.test", "pagination")
	seedOfficialKeywordAsDaily(t, context.Background(), pool, f.keyword, "page-"+f.keyword.String(), day)
	f.svc = readmodelapp.NewService(readmodelrepo.NewRepository(readmodeldb.New(pool), pool), platformoss.NewURLResolver(platformoss.URLResolverConfig{PublicBaseURL: "https://cdn.test.local"}), nil, nil)
	return f
}

// Shared UUIDs across these three tables make targeted cover corruption explicit.
func (f *publicPageFixture) insert(t *testing.T, published time.Time, keys []float64) []uuid.UUID {
	t.Helper()
	ctx := context.Background()
	assets, uploads, images := make([][]any, 0, len(keys)), make([][]any, 0, len(keys)), make([][]any, 0, len(keys))
	contents := make([][]any, 0, len(keys))
	ids := make([]uuid.UUID, 0, len(keys))
	for i, key := range keys {
		id := uuid.New()
		ids = append(ids, id)
		assets = append(assets, []any{id, f.user, "image", "image/jpeg", id.String() + ".jpg", int64(1024), int32(1200), int32(900), "ready"})
		uploads = append(uploads, []any{id, f.user, "official_today", f.keyword, f.day, "visible", id, int32(1), key, published.Add(time.Duration(i/3) * time.Microsecond)})
		images = append(images, []any{id, id, id, int32(1)})
		contents = append(contents, []any{id, "作品标题", "作品说明"})
	}
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, table := range []struct {
		name    string
		columns []string
		rows    [][]any
	}{
		{"media_assets", []string{"id", "owner_user_id", "media_kind", "mime_type", "original_object_key", "byte_size", "width", "height", "status"}, assets},
		{"work_uploads", []string{"id", "author_user_id", "context_type", "official_keyword_id", "biz_date", "visibility_status", "cover_asset_id", "image_count", "rand_key", "published_at"}, uploads},
		{"work_upload_images", []string{"id", "upload_id", "image_asset_id", "display_order"}, images},
		{"work_upload_image_contents", []string{"work_upload_image_id", "title", "note"}, contents},
	} {
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{table.name}, table.columns, pgx.CopyFromRows(table.rows)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return ids
}

func (f *publicPageFixture) page(t *testing.T, scope string, options readmodelapp.PublicUploadListOptions) *readmodelapp.PublicUploadListOutput {
	t.Helper()
	var out *readmodelapp.PublicUploadListOutput
	var err error
	if scope == "date" {
		out, err = f.svc.ListOfficialDateUploads(context.Background(), f.day, options, nil)
	} else {
		out, err = f.svc.ListReviewAllUploadsByKeyword(context.Background(), f.user, f.keyword, options)
	}
	if err != nil {
		t.Fatal(err)
	}
	if out.Items == nil || out.HasMore != (out.NextCursor != nil) {
		t.Fatalf("invalid page metadata: %+v", out)
	}
	return out
}

const publicBaselineFrom = ` FROM work_uploads wu
JOIN work_upload_images ci ON ci.upload_id=wu.id AND ci.image_asset_id=wu.cover_asset_id AND ci.deleted_at IS NULL
LEFT JOIN work_upload_image_contents content ON content.work_upload_image_id=ci.id
JOIN media_assets ca ON ca.id=wu.cover_asset_id AND ca.deleted_at IS NULL
WHERE wu.context_type='official_today' AND wu.visibility_status='visible' AND wu.deleted_at IS NULL`

func (f *publicPageFixture) expected(t *testing.T, scope, sort string, seed float64) []uuid.UUID {
	t.Helper()
	query := "SELECT wu.id" + publicBaselineFrom
	var arg any = f.day
	if scope == "date" {
		query += " AND wu.biz_date=$1"
	} else {
		query += " AND wu.official_keyword_id=$1"
		arg = f.keyword
	}
	args := []any{arg}
	if sort == "latest" {
		query += " ORDER BY wu.published_at DESC,wu.id DESC"
	} else {
		query += " ORDER BY CASE WHEN wu.rand_key < $2::double precision THEN 1 ELSE 0 END,wu.rand_key,wu.id"
		args = append(args, seed)
	}
	rows, err := f.pool.Query(context.Background(), query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		t.Fatal(err)
	}
	return ids
}

func (f *publicPageFixture) traverse(t *testing.T, scope, sort string, seed float64, limit int) []uuid.UUID {
	t.Helper()
	opts := readmodelapp.PublicUploadListOptions{Sort: &sort, Seed: &seed, Limit: &limit}
	all := make([]uuid.UUID, 0)
	for page := 0; page < 1000; page++ {
		out := f.page(t, scope, opts)
		if len(out.Items) > limit {
			t.Fatal("limit exceeded")
		}
		if sort == "random" && (out.Seed == nil || *out.Seed != seed) {
			t.Fatal("seed changed")
		}
		if sort == "latest" && out.Seed != nil {
			t.Fatal("latest seed must be null")
		}
		for _, item := range out.Items {
			all = append(all, item.ID)
		}
		if !out.HasMore {
			return all
		}
		if opts.Cursor != nil && *opts.Cursor == *out.NextCursor {
			t.Fatal("cursor did not advance")
		}
		opts = readmodelapp.PublicUploadListOptions{Cursor: out.NextCursor, Limit: &limit}
	}
	t.Fatal("pagination did not terminate")
	return nil
}

func TestPublicUploadsFullTraversal(t *testing.T) {
	pool := openIntegrationDB(t)
	for index, count := range []int{0, 1, 19, 20, 21, 40, 41, 51, 101} {
		f := newPublicPageFixture(t, pool, time.Date(2020, 1, 1+index, 0, 0, 0, 0, time.UTC))
		keys := make([]float64, count)
		for i := range keys {
			keys[i] = float64(i%11) / 11
		}
		f.insert(t, f.day, keys)
		for _, scope := range []string{"date", "keyword"} {
			for _, sort := range []string{"latest", "random"} {
				for _, limit := range []int{1, 20, 50} {
					t.Run(fmt.Sprintf("%d/%s/%s/%d", count, scope, sort, limit), func(t *testing.T) {
						want := f.expected(t, scope, sort, 0.5)
						got := f.traverse(t, scope, sort, 0.5, limit)
						if len(got) != count || !reflect.DeepEqual(got, want) {
							t.Fatalf("traversal got %v want %v", got, want)
						}
					})
				}
			}
		}
	}
}

func TestPublicUploadsRandomBoundaries(t *testing.T) {
	pool := openIntegrationDB(t)
	index := 0
	for _, high := range []int{0, 19, 20, 21} {
		for _, low := range []int{0, 3} {
			index++
			f := newPublicPageFixture(t, pool, time.Date(2020, 2, index, 0, 0, 0, 0, time.UTC))
			keys := make([]float64, high+low)
			for i := range keys {
				if i < high {
					keys[i] = 0.5
				} else {
					keys[i] = math.Nextafter(0.5, 0)
				}
			}
			f.insert(t, f.day, keys)
			for _, scope := range []string{"date", "keyword"} {
				for _, seed := range []float64{0, 0.5, math.Nextafter(1, 0)} {
					got := f.traverse(t, scope, "random", seed, 20)
					want := f.expected(t, scope, "random", seed)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("high=%d low=%d seed=%g", high, low, seed)
					}
				}
			}
			if high == 20 && low > 0 {
				first := f.page(t, "date", readmodelapp.PublicUploadListOptions{Sort: pagePtr("random"), Seed: pagePtr(0.5)})
				data, _ := base64.RawURLEncoding.DecodeString(*first.NextCursor)
				var cursor struct {
					Phase int `json:"phase"`
				}
				if err := json.Unmarshal(data, &cursor); err != nil {
					t.Fatal(err)
				}
				if cursor.Phase != 0 {
					t.Fatal("probe moved cursor to low phase")
				}
			}
		}
	}
}

func TestPublicUploadsRetryAndInvalidCovers(t *testing.T) {
	pool := openIntegrationDB(t)
	f := newPublicPageFixture(t, pool, time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC))
	keys := make([]float64, 70)
	for i := range keys {
		keys[i] = float64(i) / 100
	}
	ids := f.insert(t, f.day, keys)
	for i := 0; i < 25; i++ {
		query := "UPDATE work_upload_images SET deleted_at=now() WHERE id=$1"
		if i%3 == 1 {
			query = "UPDATE media_assets SET deleted_at=now() WHERE id=$1"
		} else if i%3 == 2 {
			query = "DELETE FROM work_upload_images WHERE id=$1"
		}
		if _, err := pool.Exec(context.Background(), query, ids[i]); err != nil {
			t.Fatal(err)
		}
	}
	for _, scope := range []string{"date", "keyword"} {
		for _, sort := range []string{"latest", "random"} {
			first := f.page(t, scope, readmodelapp.PublicUploadListOptions{Sort: &sort, Seed: pagePtr(0.0), Limit: pagePtr(1)})
			opts := readmodelapp.PublicUploadListOptions{Cursor: first.NextCursor, Limit: pagePtr(20)}
			second := f.page(t, scope, opts)
			retry := f.page(t, scope, opts)
			if len(second.Items) != 20 || !second.HasMore || !reflect.DeepEqual(second, retry) {
				t.Fatal("retry or JOIN pagination failed")
			}
			all := []uuid.UUID{first.Items[0].ID}
			for _, item := range second.Items {
				all = append(all, item.ID)
			}
			last := f.page(t, scope, readmodelapp.PublicUploadListOptions{Cursor: second.NextCursor, Limit: pagePtr(50)})
			for _, item := range last.Items {
				all = append(all, item.ID)
			}
			if last.HasMore || !reflect.DeepEqual(all, f.expected(t, scope, sort, 0)) {
				t.Fatal("changing limits lost records")
			}
		}
	}
}

func (f *publicPageFixture) router() http.Handler {
	return server.NewRouter(server.Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), GinMode: gin.TestMode,
		V1: &v1.Handler{ReadModel: f.svc}, AccessTokenVerifier: testAccessTokenVerifier{users: map[string]string{"viewer": f.user.String()}}})
}

func TestPublicUploadsHTTPContract(t *testing.T) {
	pool := openIntegrationDB(t)
	f := newPublicPageFixture(t, pool, time.Date(2020, 4, 1, 0, 0, 0, 0, time.UTC))
	ids := f.insert(t, f.day, []float64{0, 0.5, math.Nextafter(0.5, 1), math.Nextafter(1, 0)})
	if _, err := pool.Exec(context.Background(), "INSERT INTO work_upload_reactions (upload_id,user_id,type) VALUES ($1,$2,'inspired')", ids[0], f.user); err != nil {
		t.Fatal(err)
	}
	router := f.router()
	paths := []string{"/v1/official/dates/2020-04-01/uploads", "/v1/review/keywords/" + f.keyword.String() + "/uploads/all"}
	for _, path := range paths {
		first := performRequest(t, router, "GET", path+"?sort=random&seed=0&limit=1&include_reaction_counts=true", "viewer", nil)
		if first.Code != 200 {
			t.Fatalf("first: %d %s", first.Code, first.Body)
		}
		var payload struct {
			Items      []map[string]any `json:"items"`
			Seed       *float64         `json:"seed"`
			NextCursor *string          `json:"next_cursor"`
			HasMore    bool             `json:"has_more"`
		}
		if err := json.Unmarshal(first.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(first.Body.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		if len(fields) != 4 || !payload.HasMore || payload.NextCursor == nil || payload.Seed == nil || *payload.Seed != 0 {
			t.Fatalf("bad envelope: %s", first.Body)
		}
		if _, ok := payload.Items[0]["reaction_counts"]; !ok {
			t.Fatal("counts missing")
		}
		if !reflect.DeepEqual(payload.Items[0]["my_reactions"], []any{"inspired"}) {
			t.Fatalf("my_reactions: %+v", payload.Items)
		}
		cursor := url.QueryEscape(*payload.NextCursor)
		second := performRequest(t, router, "GET", path+"?cursor="+cursor, "viewer", nil)
		if second.Code != 200 {
			t.Fatalf("restore random: %d %s", second.Code, second.Body)
		}
		if err := json.Unmarshal(second.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.HasMore || payload.NextCursor != nil || payload.Seed == nil || *payload.Seed != 0 || len(payload.Items) != 3 {
			t.Fatalf("bad last page: %s", second.Body)
		}
		for _, tc := range []struct{ query, reason string }{
			{"cursor=", "readmodel.invalid_cursor"},
			{"cursor=broken", "readmodel.invalid_cursor"},
			{"cursor=" + strings.Repeat("a", 2049), "readmodel.invalid_cursor"},
			{"cursor=" + cursor + "&sort=latest", "readmodel.cursor_mismatch"},
			{"cursor=" + cursor + "&seed=0.3", "readmodel.cursor_mismatch"},
			{"sort=random&seed=1", "validation.failed"},
			{"sort=random&seed=-0.1", "validation.failed"},
			{"sort=random&seed=NaN", "validation.failed"},
			{"sort=random&seed=Infinity", "validation.failed"},
			{"seed=abc", "validation.failed"},
			{"limit=0", "validation.failed"}, {"limit=51", "validation.failed"}, {"sort=invalid", "validation.failed"},
		} {
			rec := performRequest(t, router, "GET", path+"?"+tc.query, "viewer", nil)
			err := decodeErrorResponse(t, rec)
			if rec.Code != 400 || err.Code != "validation" || err.Reason != tc.reason || err.RequestID == "" {
				t.Fatalf("%s: %d %s", tc.query, rec.Code, rec.Body)
			}
		}
		for _, other := range []string{paths[0] + "?cursor=" + cursor, paths[1] + "?cursor=" + cursor} {
			if strings.HasPrefix(other, path+"?") {
				continue
			}
			rec := performRequest(t, router, "GET", other, "viewer", nil)
			if rec.Code != 400 || decodeErrorResponse(t, rec).Reason != "readmodel.cursor_mismatch" {
				t.Fatalf("cross scope: %s", rec.Body)
			}
		}
		legacy := performRequest(t, router, "GET", path+"?seed=12", "viewer", nil)
		if legacy.Code != 200 {
			t.Fatalf("legacy: %s", legacy.Body)
		}
		if err := json.Unmarshal(legacy.Body.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		if string(fields["seed"]) != "null" || string(fields["next_cursor"]) != "null" || string(fields["has_more"]) != "false" {
			t.Fatalf("nullable fields: %s", legacy.Body)
		}
		generated := performRequest(t, router, "GET", path+"?sort=random&limit=1", "viewer", nil)
		if generated.Code != 200 {
			t.Fatalf("generated seed: %s", generated.Body)
		}
		if err := json.Unmarshal(generated.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Seed == nil || *payload.Seed < 0 || *payload.Seed >= 1 || payload.NextCursor == nil {
			t.Fatal("invalid generated seed")
		}
		generatedSeed := *payload.Seed
		continued := performRequest(t, router, "GET", path+"?cursor="+url.QueryEscape(*payload.NextCursor), "viewer", nil)
		if continued.Code != 200 {
			t.Fatalf("generated seed continuation: %s", continued.Body)
		}
		if err := json.Unmarshal(continued.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Seed == nil || *payload.Seed != generatedSeed {
			t.Fatal("generated seed was not restored")
		}
	}
	anon := performRequest(t, router, "GET", paths[0], "", nil)
	if anon.Code != 200 {
		t.Fatal("official feed must remain public")
	}
	var anonPayload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(anon.Body.Bytes(), &anonPayload); err != nil {
		t.Fatal(err)
	}
	for _, item := range anonPayload.Items {
		if value := item["my_reactions"]; value != nil {
			t.Fatal("anonymous viewer received personal reactions")
		}
		if value := item["reaction_counts"]; value != nil {
			t.Fatal("reaction counts should be absent by default")
		}
	}
	otherUser := uuid.New()
	seedUser(t, context.Background(), pool, otherUser, "other@pagination.test", "other")
	otherPage, err := f.svc.ListOfficialDateUploads(context.Background(), f.day, readmodelapp.PublicUploadListOptions{IncludeReactionCounts: true}, &otherUser)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range otherPage.Items {
		if len(item.MyReactions) != 0 || item.ReactionCounts == nil {
			t.Fatal("viewer or counts leaked between requests")
		}
	}
	if rec := performRequest(t, router, "GET", paths[1], "", nil); rec.Code != 401 {
		t.Fatal("keyword feed must require login")
	}
	if rec := performRequest(t, router, "GET", "/v1/official/dates/1999-01-01/uploads", "", nil); rec.Code != 404 {
		t.Fatalf("missing date: %s", rec.Body)
	}
	if rec := performRequest(t, router, "GET", "/v1/review/keywords/"+uuid.NewString()+"/uploads/all", "viewer", nil); rec.Code != 404 {
		t.Fatalf("missing keyword: %s", rec.Body)
	}
	empty := newPublicPageFixture(t, pool, time.Date(2020, 4, 2, 0, 0, 0, 0, time.UTC))
	rec := performRequest(t, empty.router(), "GET", "/v1/official/dates/2020-04-02/uploads?sort=random&seed=0.3", "", nil)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || len(fields) != 4 || string(fields["items"]) != "[]" || string(fields["seed"]) != "0.3" || string(fields["next_cursor"]) != "null" || string(fields["has_more"]) != "false" {
		t.Fatalf("empty: %s", rec.Body)
	}
}

func TestPublicUploadsChangesBetweenPages(t *testing.T) {
	pool := openIntegrationDB(t)
	for i, sort := range []string{"latest", "random"} {
		f := newPublicPageFixture(t, pool, time.Date(2020, 5, i+1, 0, 0, 0, 0, time.UTC))
		f.insert(t, f.day, []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7})
		ordered := f.expected(t, "date", sort, 0)
		exec := func(query string, args ...any) {
			t.Helper()
			if _, err := pool.Exec(context.Background(), query, args...); err != nil {
				t.Fatal(err)
			}
		}
		// One hidden row behind the first-page boundary, one ahead of it.
		exec("UPDATE work_uploads SET visibility_status='hidden' WHERE id=ANY($1::uuid[])", []uuid.UUID{ordered[0], ordered[6]})
		first := f.page(t, "date", readmodelapp.PublicUploadListOptions{Sort: &sort, Seed: pagePtr(0.0), Limit: pagePtr(2)})
		var cursor struct {
			Cutoff string `json:"cutoff_at"`
		}
		data, _ := base64.RawURLEncoding.DecodeString(*first.NextCursor)
		if err := json.Unmarshal(data, &cursor); err != nil {
			t.Fatal(err)
		}
		cutoff, err := time.Parse(time.RFC3339Nano, cursor.Cutoff)
		if err != nil {
			t.Fatal(err)
		}
		f.insert(t, cutoff.Add(time.Microsecond), []float64{0.35})                      // Excluded from this round, visible on refresh.
		exec("UPDATE work_uploads SET deleted_at=now() WHERE id=$1", first.Items[1].ID) // Cursor row may disappear.
		exec("UPDATE work_uploads SET visibility_status='hidden' WHERE id=$1", ordered[3])
		exec("UPDATE work_uploads SET deleted_at=now() WHERE id=$1", ordered[4])
		exec("UPDATE work_uploads SET visibility_status='visible' WHERE id=ANY($1::uuid[])", []uuid.UUID{ordered[0], ordered[6]})
		second := f.page(t, "date", readmodelapp.PublicUploadListOptions{Cursor: first.NextCursor, Limit: pagePtr(50)})
		got := make([]uuid.UUID, 0)
		for _, item := range second.Items {
			got = append(got, item.ID)
		}
		if second.HasMore || !reflect.DeepEqual(got, []uuid.UUID{ordered[5], ordered[6], ordered[7]}) {
			t.Fatalf("changes/%s got %v", sort, got)
		}
		refreshed := f.traverse(t, "date", sort, 0, 20)
		if len(refreshed) != 6 {
			t.Fatalf("refresh/%s: %d", sort, len(refreshed))
		}
	}
}

func TestPublicUploadsCutoffUsesDatabaseClock(t *testing.T) {
	pool := openIntegrationDB(t)
	f := newPublicPageFixture(t, pool, time.Date(2020, 5, 10, 0, 0, 0, 0, time.UTC))
	f.insert(t, f.day, []float64{0.1, 0.2})
	f.svc = readmodelapp.NewService(readmodelrepo.NewRepository(readmodeldb.New(pool), pool),
		platformoss.NewURLResolver(platformoss.URLResolverConfig{PublicBaseURL: "https://cdn.test.local"}), nil,
		func() time.Time { return time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC) })
	var before, after time.Time
	if err := pool.QueryRow(context.Background(), "SELECT clock_timestamp()").Scan(&before); err != nil {
		t.Fatal(err)
	}
	page := f.page(t, "date", readmodelapp.PublicUploadListOptions{Limit: pagePtr(1)})
	if err := pool.QueryRow(context.Background(), "SELECT clock_timestamp()").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if page.NextCursor == nil {
		t.Fatal("application clock incorrectly excluded published records")
	}
	var cursor struct {
		Cutoff string `json:"cutoff_at"`
	}
	data, err := base64.RawURLEncoding.DecodeString(*page.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &cursor); err != nil {
		t.Fatal(err)
	}
	cutoff, err := time.Parse(time.RFC3339Nano, cursor.Cutoff)
	if err != nil || cutoff.Before(before) || cutoff.After(after) {
		t.Fatalf("cutoff is not database time: %s %v", cursor.Cutoff, err)
	}
}

func TestPublicUploadsLateCommitAndBackfill(t *testing.T) {
	pool := openIntegrationDB(t)
	ctx := context.Background()
	for i, sort := range []string{"latest", "random"} {
		f := newPublicPageFixture(t, pool, time.Date(2020, 5, 20+i, 0, 0, 0, 0, time.UTC))
		f.insert(t, f.day, []float64{0.3, 0.5})
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = tx.Rollback(ctx) }()
		pending := uuid.New()
		if _, err := tx.Exec(ctx, `INSERT INTO media_assets(id,owner_user_id,media_kind,mime_type,original_object_key,byte_size,status)
VALUES ($1,$2,'image','image/jpeg',$3,1024,'ready')`, pending, f.user, pending.String()+".jpg"); err != nil {
			t.Fatal(err)
		}
		// published_at uses now(): the transaction time predates the first page,
		// even though the row becomes visible only after the first page returns.
		if _, err := tx.Exec(ctx, `INSERT INTO work_uploads(id,author_user_id,context_type,official_keyword_id,biz_date,visibility_status,cover_asset_id,image_count,rand_key)
VALUES ($1,$2,'official_today',$3,$4,'visible',$1,1,0.9)`, pending, f.user, f.keyword, f.day); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO work_upload_images(id,upload_id,image_asset_id,display_order) VALUES ($1,$1,$1,1)`, pending); err != nil {
			t.Fatal(err)
		}
		first := f.page(t, "date", readmodelapp.PublicUploadListOptions{Sort: &sort, Seed: pagePtr(0.0), Limit: pagePtr(1)})
		if first.NextCursor == nil {
			t.Fatal("expected continuation")
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		backfill := f.insert(t, f.day.Add(-time.Microsecond), []float64{0.8})[0]
		second := f.page(t, "date", readmodelapp.PublicUploadListOptions{Cursor: first.NextCursor})
		seen := make(map[uuid.UUID]bool)
		for _, item := range second.Items {
			seen[item.ID] = true
		}
		if !seen[backfill] || seen[pending] != (sort == "random") || second.HasMore {
			t.Fatalf("late commit/backfill %s: %+v", sort, seen)
		}
		if len(f.traverse(t, "date", sort, 0, 20)) != 4 {
			t.Fatal("refresh must include the late commit")
		}
	}
}

type pageQueryMarker struct{}
type pageBarrierTracer struct {
	once            sync.Once
	reached, resume chan struct{}
}

func (b *pageBarrierTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "-- name: ListPublicUploadsByDateRandomHigh :many") {
		return context.WithValue(ctx, pageQueryMarker{}, true)
	}
	return ctx
}
func (b *pageBarrierTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if ctx.Value(pageQueryMarker{}) == true {
		b.once.Do(func() {
			close(b.reached)
			select {
			case <-b.resume:
			case <-ctx.Done():
			}
		})
	}
}

func TestPublicUploadsRandomPageSnapshot(t *testing.T) {
	pool := openIntegrationDB(t)
	f := newPublicPageFixture(t, pool, time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC))
	ids := f.insert(t, f.day, []float64{0.7, 0.1})
	barrier := &pageBarrierTracer{reached: make(chan struct{}), resume: make(chan struct{})}
	config := pool.Config()
	config.ConnConfig.Tracer = barrier
	traced, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer traced.Close()
	r := readmodelrepo.NewRepository(readmodeldb.New(traced), traced)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type result struct {
		rows []readmodelrepo.PublicUploadPageRow
		err  error
	}
	done := make(chan result, 1)
	go func() {
		rows, err := r.ListPublicUploadsByDatePage(ctx, f.day, readmodelrepo.PublicUploadPageParams{Sort: "random", Seed: 0.5, CutoffAt: time.Now(), Limit: 3})
		done <- result{rows, err}
	}()
	select {
	case <-barrier.reached:
	case <-ctx.Done():
		t.Fatal("high query did not reach barrier")
	}
	_, updateErr := pool.Exec(ctx, "UPDATE work_uploads SET visibility_status='hidden' WHERE id=$1", ids[1])
	close(barrier.resume)
	if updateErr != nil {
		t.Fatal(updateErr)
	}
	got := <-done
	if got.err != nil || len(got.rows) != 2 || got.rows[1].Card.ID != ids[1] {
		t.Fatalf("mixed read views: %+v %v", got.rows, got.err)
	}
	fresh := f.page(t, "date", readmodelapp.PublicUploadListOptions{Sort: pagePtr("random"), Seed: pagePtr(0.5)})
	if len(fresh.Items) != 1 {
		t.Fatal("next request must observe the hide")
	}
}

func TestPublicUploadsIndexMigration(t *testing.T) {
	pool := openIntegrationDB(t)
	ctx := context.Background()
	check := func(name, column string, present bool) {
		t.Helper()
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_indexes WHERE indexname=$1", name).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if (count == 1) != present {
			t.Fatalf("%s present=%d want=%t", name, count, present)
		}
		if !present {
			return
		}
		var definition string
		var valid, ready bool
		if err := pool.QueryRow(ctx, "SELECT pg_get_indexdef(i.indexrelid),i.indisvalid,i.indisready FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relname=$1", name).Scan(&definition, &valid, &ready); err != nil {
			t.Fatal(err)
		}
		for _, part := range []string{"(" + column + ", rand_key, id)", "official_today", "visible", "deleted_at IS NULL"} {
			if !strings.Contains(definition, part) {
				t.Fatalf("wrong index: %s", definition)
			}
		}
		if !valid || !ready {
			t.Fatal("index is not ready and valid")
		}
	}
	for _, direction := range []string{"up", "down", "up", "up"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "db", "migrations", "000007_public_upload_pagination."+direction+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			t.Fatal(err)
		}
		check("idx_work_uploads_public_date_rand", "biz_date", direction == "up")
		check("idx_work_uploads_public_keyword_rand", "official_keyword_id", direction == "up")
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_indexes WHERE indexname IN ('idx_work_uploads_visibility_rand','idx_work_uploads_visibility_biz_date_published','idx_work_uploads_official_keyword_visibility_published')").Scan(&count); err != nil || count != 3 {
			t.Fatalf("old indexes changed: %d %v", count, err)
		}
	}
}
