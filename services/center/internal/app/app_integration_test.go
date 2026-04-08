package app

import (
	"context"
	"net/http"
	"testing"
	"time"

	"mare/services/center/internal/config"
)

func TestNewServerStartsAgainstDevelopmentDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := config.Config{
		HTTPAddr:         ":18080",
		DatabaseURL:      "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable",
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
