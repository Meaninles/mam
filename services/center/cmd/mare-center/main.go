package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"mare/services/center/internal/app"
	"mare/services/center/internal/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	application, err := app.NewServer(ctx, cfg)
	if err != nil {
		log.Fatalf("bootstrap server: %v", err)
	}
	defer application.Close(context.Background())

	if err := application.Run(ctx); err != nil && err != http.ErrServerClosed {
		log.Fatalf("run server: %v", err)
	}
}
