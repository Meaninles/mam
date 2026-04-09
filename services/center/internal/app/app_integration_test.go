package app

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/config"
)

func TestNewServerStartsAgainstDevelopmentDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	databaseURL := isolatedSchemaDatabaseURL(t, ctx)

	cfg := config.Config{
		HTTPAddr:         ":18080",
		DatabaseURL:      databaseURL,
		AppEnv:           "development",
		LogLevel:         "info",
		ServiceName:      "mare-center",
		ServiceVersion:   "dev",
		AutoMigrate:      true,
		HeartbeatTimeout: 45 * time.Second,
	}

	application, err := NewServer(ctx, cfg)
	if err != nil {
		t.Fatalf("bootstrap application: %v", err)
	}
	defer application.Close(context.Background())

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- application.Run(ctx)
	}()

	time.Sleep(500 * time.Millisecond)

	response, err := http.Get("http://127.0.0.1:18080/healthz")
	if err != nil {
		t.Fatalf("call healthz: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.StatusCode)
	}
}

func isolatedSchemaDatabaseURL(t *testing.T, ctx context.Context) string {
	t.Helper()

	baseURL := "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable"
	schema := fmt.Sprintf("test_app_%d", time.Now().UnixNano())

	adminPool, err := pgxpool.New(ctx, baseURL)
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

	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse database url: %v", err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
