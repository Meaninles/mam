package main

import (
	"context"
	"log"

	"mare/services/center/internal/app"
	"mare/services/center/internal/config"
)

func main() {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := app.RunMigrations(context.Background(), cfg); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
}
