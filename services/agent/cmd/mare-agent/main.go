package main

import (
	"log"

	"mare/services/agent/internal/app"
	"mare/services/agent/internal/config"
	"mare/services/agent/internal/logging"
)

func main() {
	ctx, stop := app.SignalContext()
	defer stop()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger := logging.New(cfg.LogLevel)
	application, err := app.NewApplication(cfg, logger)
	if err != nil {
		log.Fatalf("bootstrap agent: %v", err)
	}

	if err := application.Run(ctx); err != nil {
		log.Fatalf("run agent: %v", err)
	}
}
