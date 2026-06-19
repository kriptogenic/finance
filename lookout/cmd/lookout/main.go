package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"finance/lookout/internal/app"
	"finance/lookout/internal/config"
	"finance/lookout/internal/delivery"
	"finance/lookout/internal/pairing"
	"finance/lookout/internal/parser"
	"finance/lookout/internal/recon"
	"finance/lookout/internal/store"
	"finance/lookout/internal/telegram"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log, err := newLogger(cfg.LogLevel)
	if err != nil {
		return err
	}
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	p := parser.New(cfg.Location())

	buffer := pairing.New(cfg.TransferPairWindow, cfg.TransferHoldDuration)

	poster, err := delivery.New(cfg.FinanceAPIURL, cfg.FinanceAPIToken, &http.Client{Timeout: 30 * time.Second}, delivery.Config{}, log.Named("delivery"))
	if err != nil {
		return err
	}

	st, err := store.Open(cfg.StateFile)
	if err != nil {
		return err
	}

	rc := recon.New(log.Named("recon"))

	orchestrator := app.New(p, buffer, poster, poster, st, rc, cfg.PollInterval, log.Named("app"))

	source := telegram.New(telegram.Config{
		APIID:              cfg.TelegramAPIID,
		APIHash:            cfg.TelegramAPIHash,
		SessionFile:        cfg.SessionFile,
		SourceBot:          cfg.SourceBot,
		Phone:              cfg.TelegramPhone,
		AuthMode:           cfg.AuthMode,
		PollInterval:       cfg.PollInterval,
		Location:           cfg.Location(),
		BalanceSendOnStart: cfg.BalanceSendOnStart,
	}, log.Named("telegram"), nil, nil)

	log.Info("lookout starting",
		zap.String("source_bot", cfg.SourceBot),
		zap.String("finance_api", cfg.FinanceAPIURL),
		zap.String("timezone", cfg.Timezone),
	)

	if err := source.Run(ctx, orchestrator.Run); err != nil && ctx.Err() == nil {
		return err
	}
	log.Info("lookout stopped")
	return nil
}

func newLogger(level string) (*zap.Logger, error) {
	var lvl zap.AtomicLevel
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("invalid LOG_LEVEL %q: %w", level, err)
	}
	cfg := zap.NewProductionConfig()
	cfg.Level = lvl
	return cfg.Build()
}
