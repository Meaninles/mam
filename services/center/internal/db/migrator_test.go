package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigratorApplyIsIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := NewMigrator()

	firstState, err := migrator.Apply(ctx, pool)
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}

	if firstState.CurrentVersion != 1 || firstState.LatestVersion != 1 || firstState.Status != "ready" {
		t.Fatalf("unexpected first state: %+v", firstState)
	}

	secondState, err := migrator.Apply(ctx, pool)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}

	if secondState.CurrentVersion != 1 || secondState.LatestVersion != 1 || secondState.Status != "ready" {
		t.Fatalf("unexpected second state: %+v", secondState)
	}
}

func TestMigratorStateReportsPendingWithoutApply(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool := openTestPool(t, ctx)
	defer pool.Close()
	resetSchema(t, ctx, pool)

	migrator := NewMigrator()
	state, err := migrator.State(ctx, pool)
	if err != nil {
		t.Fatalf("state: %v", err)
	}

	if state.Status != "pending" {
		t.Fatalf("expected pending state, got %+v", state)
	}
	if state.CurrentVersion != 0 || state.LatestVersion != 1 {
		t.Fatalf("unexpected pending version state: %+v", state)
	}
}

func openTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	pool, err := Open(ctx, developmentDatabaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return pool
}

func resetSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS agents"); err != nil {
		t.Fatalf("drop agents: %v", err)
	}
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS schema_migrations"); err != nil {
		t.Fatalf("drop schema_migrations: %v", err)
	}
}
