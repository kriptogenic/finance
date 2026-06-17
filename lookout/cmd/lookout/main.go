package main

import (
	"context"
	"errors"
	"flag"
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
	var (
		signIn      = flag.Bool("sign-in", false, "interactively authenticate Telegram (scan QR / enter 2FA), write the session file, then exit")
		waitSession = flag.Bool("wait-session", false, "wait for the Telegram session file to exist before starting the poll loop (deploy mode)")
	)
	flag.Parse()

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

	source := telegram.New(telegram.Config{
		APIID:        cfg.TelegramAPIID,
		APIHash:      cfg.TelegramAPIHash,
		SessionFile:  cfg.SessionFile,
		SourceBot:    cfg.SourceBot,
		Phone:        cfg.TelegramPhone,
		AuthMode:     cfg.AuthMode,
		PollInterval: cfg.PollInterval,
	}, log.Named("telegram"), nil, nil)

	if *signIn {
		log.Info("sign-in mode: authenticate by hand", zap.String("auth_mode", cfg.AuthMode))
		if err := source.SignIn(ctx); err != nil && ctx.Err() == nil {
			return err
		}
		return nil
	}

	if *waitSession {
		if err := waitForSession(ctx, log, cfg.SessionFile); err != nil {
			return err
		}
	}

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

	orchestrator := app.New(p, buffer, poster, st, rc, cfg.PollInterval, log.Named("app"))

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

const sessionWaitInterval = 10 * time.Second

func waitForSession(ctx context.Context, log *zap.Logger, path string) error {
	for {
		switch _, err := os.Stat(path); {
		case err == nil:
			log.Info("session file found; starting", zap.String("session_file", path))
			return nil
		case !errors.Is(err, os.ErrNotExist):
			return fmt.Errorf("stat session file %q: %w", path, err)
		}

		log.Info("no session file yet; waiting for sign-in",
			zap.String("session_file", path),
			zap.Duration("retry_in", sessionWaitInterval),
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sessionWaitInterval):
		}
	}
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
