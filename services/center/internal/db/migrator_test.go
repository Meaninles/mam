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

	if firstState.CurrentVersion != 2 || firstState.LatestVersion != 2 || firstState.Status != "ready" {
		t.Fatalf("unexpected first state: %+v", firstState)
	}

	secondState, err := migrator.Apply(ctx, pool)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}

	if secondState.CurrentVersion != 2 || secondState.LatestVersion != 2 || secondState.Status != "ready" {
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
	if state.CurrentVersion != 0 || state.LatestVersion != 2 {
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

	if _, err := pool.Exec(ctx, `
		DROP TABLE IF EXISTS mount_scan_histories;
		DROP TABLE IF EXISTS mount_runtime;
		DROP TABLE IF EXISTS mounts;
		DROP TABLE IF EXISTS storage_node_runtime;
		DROP TABLE IF EXISTS storage_node_credentials;
		DROP TABLE IF EXISTS storage_nodes;
		DROP TABLE IF EXISTS agents;
		DROP TABLE IF EXISTS schema_migrations;
	`); err != nil {
		t.Fatalf("drop schema_migrations: %v", err)
	}
}
