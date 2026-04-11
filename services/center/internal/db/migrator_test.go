package db

import (
	"context"
	"fmt"
	"net/url"
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
	files, err := loadMigrationFiles()
	if err != nil {
		t.Fatalf("load migration files: %v", err)
	}
	expectedLatest := files[len(files)-1].Version

	firstState, err := migrator.Apply(ctx, pool)
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}

	if firstState.CurrentVersion != expectedLatest || firstState.LatestVersion != expectedLatest || firstState.Status != "ready" {
		t.Fatalf("unexpected first state: %+v", firstState)
	}

	secondState, err := migrator.Apply(ctx, pool)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}

	if secondState.CurrentVersion != expectedLatest || secondState.LatestVersion != expectedLatest || secondState.Status != "ready" {
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
	files, err := loadMigrationFiles()
	if err != nil {
		t.Fatalf("load migration files: %v", err)
	}
	expectedLatest := files[len(files)-1].Version
	state, err := migrator.State(ctx, pool)
	if err != nil {
		t.Fatalf("state: %v", err)
	}

	if state.Status != "pending" {
		t.Fatalf("expected pending state, got %+v", state)
	}
	if state.CurrentVersion != 0 || state.LatestVersion != expectedLatest {
		t.Fatalf("unexpected pending version state: %+v", state)
	}
}

func openTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	pool, err := Open(ctx, isolatedSchemaDatabaseURL(t, ctx))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return pool
}

func isolatedSchemaDatabaseURL(t *testing.T, ctx context.Context) string {
	t.Helper()

	schema := fmt.Sprintf("test_db_%d", time.Now().UnixNano())
	adminPool, err := pgxpool.New(ctx, developmentDatabaseURL)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	t.Cleanup(adminPool.Close)

	if _, err := adminPool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS "`+schema+`"`); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = adminPool.Exec(cleanupCtx, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`)
	})

	parsed, err := url.Parse(developmentDatabaseURL)
	if err != nil {
		t.Fatalf("parse database url: %v", err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func resetSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
		DROP TABLE IF EXISTS directory_tag_links;
		DROP TABLE IF EXISTS asset_tag_links;
		DROP TABLE IF EXISTS job_object_links;
		DROP TABLE IF EXISTS job_events;
		DROP TABLE IF EXISTS job_attempts;
		DROP TABLE IF EXISTS job_items;
		DROP TABLE IF EXISTS jobs;
		DROP TABLE IF EXISTS tag_library_scopes;
		DROP TABLE IF EXISTS tags;
		DROP TABLE IF EXISTS tag_groups;
		DROP TABLE IF EXISTS asset_metadata;
		DROP TABLE IF EXISTS directory_presences;
		DROP TABLE IF EXISTS asset_replicas;
		DROP TABLE IF EXISTS assets;
		DROP TABLE IF EXISTS library_directories;
		DROP TABLE IF EXISTS libraries;
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
