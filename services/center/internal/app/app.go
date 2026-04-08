package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"mare/services/center/internal/agentregistry"
	"mare/services/center/internal/config"
	"mare/services/center/internal/db"
	httpapi "mare/services/center/internal/http"
	"mare/services/center/internal/logging"
	"mare/services/center/internal/runtime"
	"mare/services/center/internal/storage"
)

type ServerApplication struct {
	config     config.Config
	logger     *slog.Logger
	httpServer *http.Server
	dbPool     interface {
		Close()
	}
}

func NewServer(ctx context.Context, cfg config.Config) (*ServerApplication, error) {
	logger := logging.New(cfg.LogLevel).With(
		slog.String("service", cfg.ServiceName),
		slog.String("version", cfg.ServiceVersion),
	)

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	migrator := db.NewMigrator()
	if cfg.AutoMigrate {
		if _, err := migrator.Apply(ctx, pool); err != nil {
			pool.Close()
			return nil, err
		}
	}

	agentService := agentregistry.NewService(pool)
	localFolderService := storage.NewLocalFolderService(pool)
	runtimeService := runtime.NewService(
		cfg.ServiceName,
		cfg.ServiceVersion,
		time.Now(),
		cfg.HeartbeatTimeout,
		pool,
		migrator,
		agentService,
	)

	router := httpapi.NewRouter(httpapi.Dependencies{
		Logger:       logger,
		Runtime:      runtimeService,
		Agents:       agentService,
		LocalNodes:   localFolderService,
		LocalFolders: localFolderService,
	})

	return &ServerApplication{
		config: cfg,
		logger: logger,
		httpServer: &http.Server{
			Addr:              cfg.HTTPAddr,
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second,
		},
		dbPool: pool,
	}, nil
}

func (a *ServerApplication) Run(ctx context.Context) error {
	serverErrors := make(chan error, 1)

	go func() {
		serverErrors <- a.httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return a.httpServer.Shutdown(shutdownCtx)
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (a *ServerApplication) Close(ctx context.Context) error {
	if a.dbPool != nil {
		a.dbPool.Close()
	}
	return nil
}

func RunMigrations(ctx context.Context, cfg config.Config) error {
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	migrator := db.NewMigrator()
	_, err = migrator.Apply(ctx, pool)
	return err
}
