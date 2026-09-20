package integration

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	officialapp "cixing/internal/modules/official/application"
	dofficial "cixing/internal/modules/official/domain"
	"cixing/internal/shared/common"
)

func TestOfficialKeywordChangesProjectPendingState(t *testing.T) {
	for _, initiallyActive := range []bool{true, false} {
		for _, delay := range []int{0, 1} {
			t.Run(fmt.Sprintf("initially_active_%t_delay_%d", initiallyActive, delay), func(t *testing.T) {
				t.Parallel()
				ctx := context.Background()
				pool := openIntegrationDB(t)
				day := dateOnly(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC))
				now := day.Add(4 * time.Hour)
				catalog, repo := newTestCatalogService(pool, func() time.Time { return now })
				keywords, err := repo.ListActiveOfficialKeywords(ctx)
				if err != nil {
					t.Fatal(err)
				}
				keyword := keywords[0]
				if err := repo.SetOfficialKeywordActive(ctx, keyword.ID, initiallyActive); err != nil {
					t.Fatal(err)
				}
				if err := catalog.InitializeRotation(ctx, day); err != nil {
					t.Fatal(err)
				}
				queueBefore := snapshotRotationQueue(t, ctx, pool)
				params := dofficial.UpsertKeywordParams{
					ID: keyword.ID, Text: keyword.Text, Category: keyword.Category, IsActive: !initiallyActive,
				}
				for attempt := 0; attempt < 2; attempt++ {
					if _, err := catalog.ScheduleOfficialKeyword(ctx, params); err != nil {
						t.Fatalf("schedule first change: %v", err)
					}
				}
				now = now.AddDate(0, 0, delay)
				params.IsActive = initiallyActive
				for attempt := 0; attempt < 2; attempt++ {
					if _, err := catalog.ScheduleOfficialKeyword(ctx, params); err != nil {
						t.Fatalf("schedule reversal before catch-up: %v", err)
					}
				}
				pending, err := repo.ListPendingOfficialKeywordChangesThrough(ctx, day.AddDate(0, 0, delay+1))
				if err != nil {
					t.Fatal(err)
				}
				if len(pending) != 2 {
					t.Fatalf("accepted changes = %d, want exactly two (retries must not enqueue again)", len(pending))
				}
				for i, effectiveOffset := range []int{1, delay + 1} {
					if !pending[i].EffectiveDate.Equal(day.AddDate(0, 0, effectiveOffset)) {
						t.Fatalf("change %d effective date = %s", i, pending[i].EffectiveDate)
					}
				}
				current, err := repo.GetOfficialKeywordByID(ctx, keyword.ID)
				if err != nil {
					t.Fatal(err)
				}
				if current.IsActive != initiallyActive {
					t.Fatal("accepting pending changes changed the live catalog")
				}
				if !reflect.DeepEqual(queueBefore, snapshotRotationQueue(t, ctx, pool)) {
					t.Fatal("accepting or simulating changes consumed queue positions")
				}
				var assignmentCount int
				if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM daily_keyword_assignments`).Scan(&assignmentCount); err != nil {
					t.Fatal(err)
				}
				if assignmentCount != 0 {
					t.Fatal("simulation created official assignments")
				}
				if delay == 1 {
					now = day.AddDate(0, 0, 1).Add(4 * time.Hour)
					if _, err := catalog.EnsureDailyKeywordAssignment(ctx, day.AddDate(0, 0, 1)); err != nil {
						t.Fatal(err)
					}
					intermediate, err := repo.GetOfficialKeywordByID(ctx, keyword.ID)
					if err != nil {
						t.Fatal(err)
					}
					if intermediate.IsActive == initiallyActive {
						t.Fatal("first change did not take effect before the reversal")
					}
				}
				now = day.AddDate(0, 0, delay+1).Add(4 * time.Hour)
				if _, err := catalog.EnsureDailyKeywordAssignment(ctx, day.AddDate(0, 0, delay+1)); err != nil {
					t.Fatal(err)
				}
				final, err := repo.GetOfficialKeywordByID(ctx, keyword.ID)
				if err != nil {
					t.Fatal(err)
				}
				if final.IsActive != initiallyActive {
					t.Fatal("accepted reversal was lost during catch-up")
				}
			})
		}
	}
}

func TestOfficialKeywordPendingActivationsBootstrapEmptyCatalog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := openIntegrationDB(t)
	if _, err := pool.Exec(ctx, `UPDATE official_keywords SET is_active = FALSE`); err != nil {
		t.Fatal(err)
	}
	day := dateOnly(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC))
	now := day.Add(4 * time.Hour)
	catalog, repo := newTestCatalogService(pool, func() time.Time { return now })
	if err := catalog.EnsureRotationInitialized(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.EnsureDailyKeywordAssignment(ctx, day); !errors.Is(err, common.ErrNotFound) {
		t.Fatalf("empty pre-cutover catalog = %v, want not found", err)
	}
	for i := 0; i < 7; i++ {
		if _, err := catalog.ScheduleOfficialKeyword(ctx, dofficial.UpsertKeywordParams{
			ID: uuid.New(), Text: fmt.Sprintf("bootstrap-%d", i), Category: dofficial.KeywordCategoryAbstract, IsActive: true,
		}); err != nil {
			t.Fatal(err)
		}
	}
	now = now.AddDate(0, 0, 1)
	if _, err := catalog.EnsureDailyKeywordAssignment(ctx, day.AddDate(0, 0, 1)); err != nil {
		t.Fatalf("schedule first day with seven pending activations: %v", err)
	}
	active, err := repo.ListActiveOfficialKeywords(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 7 || len(snapshotRotationQueue(t, ctx, pool)) != 7 {
		t.Fatal("the seven new keywords were not activated and queued")
	}
	now = now.AddDate(0, 0, 14)
	if _, err := catalog.EnsureDailyKeywordAssignment(ctx, common.NormalizeBizDate(now)); err != nil {
		t.Fatalf("continue rotation after bootstrap: %v", err)
	}
}

func TestOfficialKeywordDeactivationChecksOnlyAffectedWindows(t *testing.T) {
	for _, repeatOffset := range []int{6, 5} {
		t.Run(fmt.Sprintf("old_repeat_starts_%d_days_ago", repeatOffset), func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			pool := openIntegrationDB(t)
			day := dateOnly(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC))
			now := day.Add(4 * time.Hour)
			catalog, repo := newTestCatalogService(pool, func() time.Time { return now })
			keywords, err := repo.ListActiveOfficialKeywords(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for offset := 6; offset >= 1; offset-- {
				index := 6 - offset
				if offset == repeatOffset-1 {
					index = 6 - repeatOffset
				}
				seedDailyKeywordAssignment(t, ctx, pool, keywords[index].ID, day.AddDate(0, 0, -offset))
			}
			if err := catalog.InitializeRotation(ctx, day); err != nil {
				t.Fatal(err)
			}
			before := snapshotOfficialRotationData(t, ctx, pool)
			err = catalog.DeactivateOfficialKeyword(ctx, keywords[20].ID)
			if repeatOffset == 5 {
				// This repeat is still inside the first window affected by S+1.
				if !errors.Is(err, officialapp.ErrKeywordChangeNotFeasible) {
					t.Fatalf("affected invalid window = %v, want infeasible change", err)
				}
				if after := snapshotOfficialRotationData(t, ctx, pool); after != before {
					t.Fatal("rejected simulation changed production data")
				}
				return
			}
			if err != nil {
				t.Fatalf("S-6/S-5 repeat only affects pre-change windows: %v", err)
			}
			now = day.AddDate(0, 0, 30).Add(4 * time.Hour)
			if _, err := catalog.EnsureDailyKeywordAssignment(ctx, common.NormalizeBizDate(now)); err != nil {
				t.Fatalf("production scheduling after feasible change: %v", err)
			}
		})
	}
}

func TestOfficialKeywordCatalogRejectsUnrecordedChanges(t *testing.T) {
	for _, scenario := range []string{"deactivate", "reactivate", "new_inactive", "missing_baseline", "missing_applied_change", "overdue_change_masks_edit"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			pool := openIntegrationDB(t)
			day := dateOnly(time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC))
			now := day.Add(4 * time.Hour)
			catalog, repo := newTestCatalogService(pool, func() time.Time { return now })
			if err := catalog.InitializeRotation(ctx, day); err != nil {
				t.Fatal(err)
			}
			if _, err := catalog.EnsureDailyKeywordAssignment(ctx, day); err != nil {
				t.Fatal(err)
			}
			keywords, err := repo.ListActiveOfficialKeywords(ctx)
			if err != nil {
				t.Fatal(err)
			}
			keywordID := keywords[1].ID
			if scenario == "reactivate" || scenario == "missing_applied_change" || scenario == "overdue_change_masks_edit" {
				if err := catalog.DeactivateOfficialKeyword(ctx, keywordID); err != nil {
					t.Fatal(err)
				}
				if scenario != "overdue_change_masks_edit" {
					now = now.AddDate(0, 0, 1)
					if _, err := catalog.EnsureDailyKeywordAssignment(ctx, common.NormalizeBizDate(now)); err != nil {
						t.Fatal(err)
					}
				}
			}
			switch scenario {
			case "deactivate", "overdue_change_masks_edit":
				_, err = pool.Exec(ctx, `UPDATE official_keywords SET is_active = FALSE WHERE id = $1`, keywordID)
			case "reactivate":
				_, err = pool.Exec(ctx, `UPDATE official_keywords SET is_active = TRUE WHERE id = $1`, keywordID)
			case "new_inactive":
				_, err = pool.Exec(ctx, `INSERT INTO official_keywords (id, text, category, is_active) VALUES ($1, 'unrecorded', 'abstract', FALSE)`, uuid.New())
			case "missing_baseline":
				_, err = pool.Exec(ctx, `DELETE FROM official_keyword_catalog_baseline WHERE keyword_id = $1`, keywordID)
			case "missing_applied_change":
				_, err = pool.Exec(ctx, `DELETE FROM official_keyword_changes WHERE keyword_id = $1`, keywordID)
			}
			if err != nil {
				t.Fatal(err)
			}
			before := snapshotOfficialRotationData(t, ctx, pool)
			now = now.AddDate(0, 0, 2)
			if _, err := catalog.EnsureDailyKeywordAssignment(ctx, common.NormalizeBizDate(now)); !errors.Is(err, officialapp.ErrRotationStateInconsistent) {
				t.Fatalf("catch-up with unrecorded catalog edit = %v, want state inconsistency", err)
			}
			if err := catalog.DeactivateOfficialKeyword(ctx, keywords[2].ID); !errors.Is(err, officialapp.ErrRotationStateInconsistent) {
				t.Fatalf("accept change with unrecorded catalog edit = %v, want state inconsistency", err)
			}
			if after := snapshotOfficialRotationData(t, ctx, pool); after != before {
				t.Fatal("failed operations changed the catalog, pending changes, queue, or assignments")
			}
		})
	}
}

func snapshotOfficialRotationData(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	var snapshot string
	err := pool.QueryRow(ctx, `SELECT jsonb_build_object(
		'state', (SELECT to_jsonb(s) FROM official_keyword_rotation_state s),
		'queue', (SELECT jsonb_agg(q ORDER BY q.keyword_id) FROM official_keyword_rotation_queue q),
		'assignments', (SELECT jsonb_agg(a ORDER BY a.biz_date) FROM daily_keyword_assignments a),
		'keywords', (SELECT jsonb_agg(k ORDER BY k.id) FROM official_keywords k),
		'baseline', (SELECT jsonb_agg(b ORDER BY b.keyword_id) FROM official_keyword_catalog_baseline b),
		'changes', (SELECT jsonb_agg(c ORDER BY c.id) FROM official_keyword_changes c)
	)::text`).Scan(&snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
