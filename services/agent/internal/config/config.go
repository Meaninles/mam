package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultHeartbeatInterval = 15 * time.Second
	defaultLogLevel          = "info"
	defaultAgentVersion      = "dev"
	defaultStateDirName      = "mare-agent"
	defaultAgentIDFileName   = "agent-id.txt"
)

type Config struct {
	CenterBaseURL     string
	HeartbeatInterval time.Duration
	LogLevel          string
	AgentVersion      string
	AgentMode         string
	AgentStateDir     string
	AgentIDFile       string
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		CenterBaseURL:     normalizeBaseURL(os.Getenv("CENTER_BASE_URL")),
		HeartbeatInterval: defaultHeartbeatInterval,
		LogLevel:          defaultIfEmpty(os.Getenv("LOG_LEVEL"), defaultLogLevel),
		AgentVersion:      defaultIfEmpty(os.Getenv("AGENT_VERSION"), defaultAgentVersion),
		AgentMode:         strings.TrimSpace(os.Getenv("AGENT_MODE")),
		AgentStateDir:     strings.TrimSpace(os.Getenv("AGENT_STATE_DIR")),
		AgentIDFile:       strings.TrimSpace(os.Getenv("AGENT_ID_FILE")),
	}

	if value := strings.TrimSpace(os.Getenv("HEARTBEAT_INTERVAL")); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse HEARTBEAT_INTERVAL: %w", err)
		}
		cfg.HeartbeatInterval = parsed
	}

	if cfg.AgentStateDir == "" {
		baseDir, err := os.UserConfigDir()
		if err != nil {
			return Config{}, fmt.Errorf("resolve user config dir: %w", err)
		}
		cfg.AgentStateDir = filepath.Join(baseDir, defaultStateDirName)
	}

	if cfg.AgentIDFile == "" {
		cfg.AgentIDFile = filepath.Join(cfg.AgentStateDir, defaultAgentIDFileName)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.CenterBaseURL == "" {
		return fmt.Errorf("CENTER_BASE_URL is required")
	}
	if c.AgentMode == "" {
		return fmt.Errorf("AGENT_MODE is required")
	}
	if c.HeartbeatInterval <= 0 {
		return fmt.Errorf("HEARTBEAT_INTERVAL must be greater than 0")
	}
	if c.AgentStateDir == "" {
		return fmt.Errorf("AGENT_STATE_DIR is required")
	}
	if c.AgentIDFile == "" {
		return fmt.Errorf("AGENT_ID_FILE is required")
	}
	return nil
}

func normalizeBaseURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func defaultIfEmpty(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
