package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mare/services/agent/internal/centerclient"
	"mare/services/agent/internal/config"
	httpapi "mare/services/agent/internal/httpapi"
	"mare/services/agent/internal/identity"
	importingagent "mare/services/agent/internal/importing"
	"mare/services/agent/internal/runtime"
)

type Application struct {
	runner     *Runner
	logger     *slog.Logger
	httpServer *http.Server
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
		AgentID:      agentID,
		Version:      cfg.AgentVersion,
		Hostname:     hostname,
		Platform:     runtime.DetectPlatform(),
		Mode:         cfg.AgentMode,
		ProcessID:    int64(os.Getpid()),
		Capabilities: []string{"localfs"},
		StartedAt:    time.Now().UTC(),
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
		CallbackBaseURL:   cfg.CallbackBaseURL,
		Capabilities:      summary.Capabilities,
		HeartbeatInterval: cfg.HeartbeatInterval,
		RetryDelay:        5 * time.Second,
	})
	importService := importingagent.NewService(importingagent.Options{
		ImportSourcePaths: cfg.ImportSourcePaths,
	})
	router := httpapi.NewRouter(httpapi.Dependencies{
		Importing: importService,
	})

	return &Application{
		runner: runner,
		logger: logger,
		httpServer: &http.Server{
			Addr:              cfg.AgentAPIAddr,
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}, nil
}

func (a *Application) Run(ctx context.Context) error {
	if a.logger != nil {
		a.logger.Info("agent starting")
	}
	serverErrors := make(chan error, 2)

	go func() {
		serverErrors <- a.httpServer.ListenAndServe()
	}()
	go func() {
		serverErrors <- a.runner.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return a.httpServer.Shutdown(shutdownCtx)
	case err := <-serverErrors:
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}
