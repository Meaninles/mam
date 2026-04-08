package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mare/services/agent/internal/centerclient"
	"mare/services/agent/internal/config"
	"mare/services/agent/internal/identity"
	"mare/services/agent/internal/runtime"
)

type Application struct {
	runner *Runner
	logger *slog.Logger
}

func NewApplication(cfg config.Config, logger *slog.Logger) (*Application, error) {
	agentID, err := identity.LoadOrCreate(cfg.AgentIDFile)
	if err != nil {
		return nil, err
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	summary, err := runtime.BuildSummary(runtime.Options{
		AgentID:   agentID,
		Version:   cfg.AgentVersion,
		Hostname:  hostname,
		Platform:  runtime.DetectPlatform(),
		Mode:      cfg.AgentMode,
		ProcessID: int64(os.Getpid()),
		StartedAt: time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}

	client := centerclient.New(cfg.CenterBaseURL, 5*time.Second)
	runner := NewRunner(client, Options{
		AgentID:           summary.AgentID,
		Version:           summary.Version,
		Hostname:          summary.Hostname,
		Platform:          summary.Platform,
		Mode:              summary.Mode,
		ProcessID:         summary.ProcessID,
		HeartbeatInterval: cfg.HeartbeatInterval,
		RetryDelay:        5 * time.Second,
	})

	return &Application{
		runner: runner,
		logger: logger,
	}, nil
}

func (a *Application) Run(ctx context.Context) error {
	if a.logger != nil {
		a.logger.Info("agent starting")
	}
	return a.runner.Run(ctx)
}

func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
