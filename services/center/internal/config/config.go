package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddr         = ":8080"
	defaultAppEnv           = "development"
	defaultLogLevel         = "info"
	defaultServiceName      = "mare-center"
	defaultServiceVersion   = "dev"
	defaultHeartbeatTimeout = 45 * time.Second
)

type Config struct {
	HTTPAddr         string
	DatabaseURL      string
	AppEnv           string
	LogLevel         string
	ServiceName      string
	ServiceVersion   string
	AutoMigrate      bool
	HeartbeatTimeout time.Duration
}

func LoadFromEnv() (Config, error) {
	cfg := Config{
		HTTPAddr:         defaultIfEmpty(os.Getenv("HTTP_ADDR"), defaultHTTPAddr),
		DatabaseURL:      strings.TrimSpace(os.Getenv("DATABASE_URL")),
		AppEnv:           defaultIfEmpty(os.Getenv("APP_ENV"), defaultAppEnv),
		LogLevel:         defaultIfEmpty(os.Getenv("LOG_LEVEL"), defaultLogLevel),
		ServiceName:      defaultIfEmpty(os.Getenv("SERVICE_NAME"), defaultServiceName),
		ServiceVersion:   defaultIfEmpty(os.Getenv("SERVICE_VERSION"), defaultServiceVersion),
		AutoMigrate:      true,
		HeartbeatTimeout: defaultHeartbeatTimeout,
	}

	if value := strings.TrimSpace(os.Getenv("AUTO_MIGRATE")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse AUTO_MIGRATE: %w", err)
		}
		cfg.AutoMigrate = parsed
	}

	if value := strings.TrimSpace(os.Getenv("HEARTBEAT_TIMEOUT")); value != "" {
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse HEARTBEAT_TIMEOUT: %w", err)
		}
		cfg.HeartbeatTimeout = parsed
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	if strings.TrimSpace(c.HTTPAddr) == "" {
		return fmt.Errorf("HTTP_ADDR is required")
	}

	if c.HeartbeatTimeout <= 0 {
		return fmt.Errorf("HEARTBEAT_TIMEOUT must be greater than 0")
	}

	return nil
}

func defaultIfEmpty(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
