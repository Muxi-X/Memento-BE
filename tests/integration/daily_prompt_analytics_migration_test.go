package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDailyPromptAnalyticsMigrationDownUp(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool := openIntegrationDB(t)
	migrationsDir := filepath.Join("..", "..", "db", "migrations")

	downSQL, err := os.ReadFile(filepath.Join(migrationsDir, "000005_daily_prompts_analytics.down.sql"))
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	upSQL, err := os.ReadFile(filepath.Join(migrationsDir, "000005_daily_prompts_analytics.up.sql"))
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}

	if _, err := pool.Exec(ctx, string(downSQL)); err != nil {
		t.Fatalf("execute down migration: %v", err)
	}
	assertTableExists(t, ctx, pool, "user_daily_prompts", false)
	assertTableExists(t, ctx, pool, "analytics_events", false)

	if _, err := pool.Exec(ctx, string(upSQL)); err != nil {
		t.Fatalf("execute up migration after down: %v", err)
	}
	assertTableExists(t, ctx, pool, "user_daily_prompts", true)
	assertTableExists(t, ctx, pool, "analytics_events", true)
}

func assertTableExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string, want bool) {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
		t.Fatalf("check table %s: %v", table, err)
	}
	if exists != want {
		t.Fatalf("table %s exists = %t, want %t", table, exists, want)
	}
}
