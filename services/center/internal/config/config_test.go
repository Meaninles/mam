package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvRejectsMissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("HTTP_ADDR", ":8080")

	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("expected missing DATABASE_URL error")
	}
}

func TestLoadFromEnvRejectsInvalidHeartbeatTimeout(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable")
	t.Setenv("HEARTBEAT_TIMEOUT", "bad-duration")

	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("expected invalid HEARTBEAT_TIMEOUT error")
	}
}

func TestLoadFromEnvUsesDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("SERVICE_VERSION", "")
	t.Setenv("AUTO_MIGRATE", "")
	t.Setenv("HEARTBEAT_TIMEOUT", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("unexpected http addr: %s", cfg.HTTPAddr)
	}

	if cfg.AppEnv != "development" {
		t.Fatalf("unexpected app env: %s", cfg.AppEnv)
	}

	if cfg.LogLevel != "info" {
		t.Fatalf("unexpected log level: %s", cfg.LogLevel)
	}

	if cfg.ServiceVersion != "dev" {
		t.Fatalf("unexpected service version: %s", cfg.ServiceVersion)
	}

	if cfg.AutoMigrate != true {
		t.Fatal("expected auto migrate default true")
	}

	if cfg.HeartbeatTimeout != 45*time.Second {
		t.Fatalf("unexpected heartbeat timeout: %s", cfg.HeartbeatTimeout)
	}
}
