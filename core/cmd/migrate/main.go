package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"finance/config"
	"finance/pkg/log"
	"finance/pkg/migrator"
)

// Usage:
//
//	go run ./cmd/migrate        # apply pending migrations (up)
//	go run ./cmd/migrate up     # same
//	go run ./cmd/migrate fresh  # drop everything and re-apply (destructive)
func main() {
	if err := godotenv.Load(".env"); err != nil {
		//nolint:forbidigo // logger not configured yet
		fmt.Println("Warning: no .env file found or unable to load it")
	}

	cfg, err := config.NewConfig()
	if err != nil {
		//nolint:forbidigo // logger not configured yet
		fmt.Println("config error:", err)
		os.Exit(1)
	}

	logger := log.NewLogger(&cfg.Log)
	defer func() { _ = logger.Sync() }()

	mg, err := migrator.New(cfg.MigrationsPath, cfg.MigrationDSN(), logger)
	if err != nil {
		logger.Fatal("migrator init", zap.Error(err))
	}
	defer mg.Close()

	action := "up"
	if len(os.Args) > 1 {
		action = os.Args[1]
	}

	switch action {
	case "up":
		err = mg.Up()
	case "fresh":
		err = mg.Fresh()
	default:
		logger.Fatal("unknown action", zap.String("action", action), zap.String("want", "up|fresh"))
	}

	if err != nil {
		logger.Fatal("migrate "+action, zap.Error(err))
	}
}
