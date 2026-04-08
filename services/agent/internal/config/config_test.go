package config

import (
	"testing"
	"time"
)

func TestLoadFromEnvRejectsMissingCenterBaseURL(t *testing.T) {
	t.Setenv("CENTER_BASE_URL", "")
	t.Setenv("AGENT_MODE", "attached")

	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("expected missing CENTER_BASE_URL error")
	}
}

func TestLoadFromEnvRejectsInvalidHeartbeatInterval(t *testing.T) {
	t.Setenv("CENTER_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("HEARTBEAT_INTERVAL", "bad-duration")
	t.Setenv("AGENT_MODE", "attached")

	if _, err := LoadFromEnv(); err == nil {
		t.Fatal("expected invalid HEARTBEAT_INTERVAL error")
	}
}

func TestLoadFromEnvUsesDefaults(t *testing.T) {
	t.Setenv("CENTER_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("AGENT_MODE", "attached")
	t.Setenv("HEARTBEAT_INTERVAL", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("AGENT_VERSION", "")
	t.Setenv("AGENT_STATE_DIR", "")
	t.Setenv("AGENT_ID_FILE", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.CenterBaseURL != "http://127.0.0.1:8080" {
		t.Fatalf("unexpected center url: %s", cfg.CenterBaseURL)
	}
	if cfg.HeartbeatInterval != 15*time.Second {
		t.Fatalf("unexpected heartbeat interval: %s", cfg.HeartbeatInterval)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("unexpected log level: %s", cfg.LogLevel)
	}
	if cfg.AgentVersion != "dev" {
		t.Fatalf("unexpected agent version: %s", cfg.AgentVersion)
	}
	if cfg.AgentMode != "attached" {
		t.Fatalf("unexpected agent mode: %s", cfg.AgentMode)
	}
}
