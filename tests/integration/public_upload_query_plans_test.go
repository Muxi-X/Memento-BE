package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	readmodeldb "cixing/internal/modules/readmodel/infra/db/gen"
	readmodelrepo "cixing/internal/modules/readmodel/infra/db/repo"
)

type capturedPageQuery struct {
	sql  string
	args []any
}
type pageCaptureTracer struct {
	mu      sync.Mutex
	queries []capturedPageQuery
}

func (c *pageCaptureTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(data.SQL, "-- name: ListPublicUploadsBy") {
		c.mu.Lock()
		c.queries = append(c.queries, capturedPageQuery{data.SQL, append([]any(nil), data.Args...)})
		c.mu.Unlock()
	}
	return ctx
}
func (*pageCaptureTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}
func (c *pageCaptureTracer) drain() []capturedPageQuery {
	c.mu.Lock()
	defer c.mu.Unlock()
	queries := c.queries
	c.queries = nil
	return queries
}

type uploadExplainNode struct {
	Type         string              `json:"Node Type"`
	Index        string              `json:"Index Name"`
	Rows         float64             `json:"Actual Rows"`
	Loops        float64             `json:"Actual Loops"`
	Filtered     float64             `json:"Rows Removed by Filter"`
	JoinFiltered float64             `json:"Rows Removed by Join Filter"`
	Hits         int64               `json:"Shared Hit Blocks"`
	Reads        int64               `json:"Shared Read Blocks"`
	Plans        []uploadExplainNode `json:"Plans"`
}
type uploadExplain struct {
	Plan        uploadExplainNode `json:"Plan"`
	ExecutionMS float64           `json:"Execution Time"`
}

func explainUploadQuery(t *testing.T, pool *pgxpool.Pool, label string, query capturedPageQuery) float64 {
	t.Helper()
	ctx := context.Background()
	var raw []byte
	if err := pool.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+query.sql, query.args...).Scan(&raw); err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	var plans []uploadExplain
	if err := json.Unmarshal(raw, &plans); err != nil || len(plans) != 1 {
		t.Fatalf("%s explain: %v", label, err)
	}
	var scans, filtered float64
	var sorts int
	var indexes []string
	var visit func(uploadExplainNode)
	visit = func(node uploadExplainNode) {
		if strings.Contains(node.Type, "Scan") {
			scans += (node.Rows + node.Filtered) * node.Loops
		}
		filtered += (node.Filtered + node.JoinFiltered) * node.Loops
		if strings.Contains(node.Type, "Sort") {
			sorts++
		}
		if node.Index != "" {
			indexes = append(indexes, node.Index)
		}
		for _, child := range node.Plans {
			visit(child)
		}
	}
	visit(plans[0].Plan)
	t.Logf("PLAN %s ms=%.3f scan_rows=%.0f filtered=%.0f sorts=%d hit=%d read=%d indexes=%s", label, plans[0].ExecutionMS, scans, filtered, sorts, plans[0].Plan.Hits, plans[0].Plan.Reads, strings.Join(indexes, ","))
	t.Logf("EXPLAIN %s %s", label, raw)
	return plans[0].ExecutionMS
}

func TestPublicUploadsQueryPlans(t *testing.T) {
	if os.Getenv("PUBLIC_UPLOADS_EXPLAIN") != "1" {
		t.Skip("set PUBLIC_UPLOADS_EXPLAIN=1 for the 1k/10k/100k execution-plan check")
	}
	for _, size := range []int{1000, 10000, 100000} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			pool := openIntegrationDB(t)
			ctx := context.Background()
			var target *publicPageFixture
			for group := 0; group < 10; group++ {
				f := newPublicPageFixture(t, pool, time.Date(2020, 7, group+1, 0, 0, 0, 0, time.UTC))
				if group == 0 {
					target = f
				}
				keys := make([]float64, size/10)
				for i := range keys {
					keys[i] = float64((i*37)%997) / 997
				}
				ids := f.insert(t, f.day, keys)
				invalid := make([]uuid.UUID, 0)
				for i := 0; i < len(ids); i += 97 {
					invalid = append(invalid, ids[i])
				}
				if _, err := pool.Exec(ctx, "UPDATE media_assets SET deleted_at=now() WHERE id=ANY($1::uuid[])", invalid); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := pool.Exec(ctx, "ANALYZE"); err != nil {
				t.Fatal(err)
			}
			capture := &pageCaptureTracer{}
			config := pool.Config()
			config.ConnConfig.Tracer = capture
			traced, err := pgxpool.NewWithConfig(ctx, config)
			if err != nil {
				t.Fatal(err)
			}
			defer traced.Close()
			r := readmodelrepo.NewRepository(readmodeldb.New(traced), traced)
			cutoff, err := r.PublicUploadsCutoff(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, scope := range []string{"date", "keyword"} {
				read := func(params readmodelrepo.PublicUploadPageParams) []readmodelrepo.PublicUploadPageRow {
					t.Helper()
					var rows []readmodelrepo.PublicUploadPageRow
					var err error
					if scope == "date" {
						rows, err = r.ListPublicUploadsByDatePage(ctx, target.day, params)
					} else {
						rows, err = r.ListPublicUploadsByKeywordPage(ctx, target.keyword, params)
					}
					if err != nil {
						t.Fatal(err)
					}
					return rows
				}
				run := func(label string, p readmodelrepo.PublicUploadPageParams) ([]readmodelrepo.PublicUploadPageRow, float64) {
					t.Helper()
					rows := read(p)
					queries := capture.drain()
					total := 0.0
					for i, query := range queries {
						total += explainUploadQuery(t, pool, fmt.Sprintf("%d/%s/%s/%d", size, scope, label, i), query)
					}
					return rows, total
				}
				latest := readmodelrepo.PublicUploadPageParams{Sort: "latest", CutoffAt: cutoff, Limit: 21}
				rows, _ := run("latest-first", latest)
				latest.After = &rows[len(rows)-1].Position
				run("latest-after", latest)
				random := readmodelrepo.PublicUploadPageParams{Sort: "random", Seed: 0.5, CutoffAt: cutoff, Limit: 21}
				rows, randomMS := run("random-first", random)
				random.After = &rows[len(rows)-1].Position
				run("random-after", random)
				// Only about ten high rows remain in a 100k fixture; this crosses the seed.
				random.Seed = 0.998
				random.After = nil
				rows, _ = run("random-cross", random)
				random.After = &rows[len(rows)-1].Position
				run("random-low-after", random)
				var scopeArg any = target.day
				scopeFilter := " AND wu.biz_date=$1"
				if scope == "keyword" {
					scopeArg = target.keyword
					scopeFilter = " AND wu.official_keyword_id=$1"
				}
				baseline := `SELECT wu.id,wu.biz_date,wu.official_keyword_id,ci.id,
COALESCE(NULLIF(btrim(content.title),''),NULLIF(btrim(content.note),'')),
CASE WHEN content.audio_asset_id IS NOT NULL THEN TRUE ELSE FALSE END,
content.audio_duration_ms,ca.original_object_key,wu.image_count,
wu.reaction_inspired_count,wu.reaction_resonated_count,wu.published_at AS created_at,wu.published_at,wu.rand_key` + publicBaselineFrom + scopeFilter + `
AND wu.published_at <= $2::timestamptz
ORDER BY CASE WHEN wu.rand_key < $3::double precision THEN 1 ELSE 0 END,wu.rand_key,wu.id LIMIT $4`
				oldMS := explainUploadQuery(t, pool, fmt.Sprintf("%d/%s/old-case", size, scope), capturedPageQuery{baseline, []any{scopeArg, cutoff, 0.5, 21}})
				t.Logf("COMPARE size=%d scope=%s old_case_ms=%.3f range_page_ms=%.3f", size, scope, oldMS, randomMS)
			}
		})
	}
}
