// Package config loads the bot's settings from the environment (§9) via
// cleanenv, mirroring core's convention. Secrets (API hash, session file, ingest
// token) come from the environment only — never from code (§8). Card→account and
// merchant→category are app-side, so no card/category config lives here (§6).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

// Config is the full bot configuration (§9).
type Config struct {
	// Telegram MTProto user-session settings.
	TelegramAPIID   int    `env:"TELEGRAM_API_ID" env-required:"true"`
	TelegramAPIHash string `env:"TELEGRAM_API_HASH" env-required:"true"`
	SessionFile     string `env:"SESSION_FILE" env-default:"session.json"`
	SourceBot       string `env:"SOURCE_BOT" env-required:"true"`

	// PollInterval is how often message history is polled (§3).
	PollInterval time.Duration `env:"POLL_INTERVAL" env-default:"60s"`

	// TransferPairWindow is the max gap between the two legs' 🕓 times to pair
	// them into a transfer (§5.1).
	TransferPairWindow time.Duration `env:"TRANSFER_PAIR_WINDOW" env-default:"120s"`
	// TransferHoldDuration is how long an unmatched leg waits before being
	// flushed standalone; must exceed PollInterval + skew (§5.1).
	TransferHoldDuration time.Duration `env:"TRANSFER_HOLD_DURATION" env-default:"5m"`

	// Ingest target.
	FinanceAPIURL   string `env:"FINANCE_API_URL" env-required:"true"`
	FinanceAPIToken string `env:"FINANCE_API_TOKEN" env-default:""`

	// StateFile holds the watermark + pending transfer legs (§8).
	StateFile string `env:"STATE_FILE" env-default:"lookout-state.json"`

	// Timezone interprets the 🕓 field; default Asia/Tashkent (§4.2).
	Timezone string `env:"TIMEZONE" env-default:"Asia/Tashkent"`

	// LogLevel for the zap logger.
	LogLevel string `env:"LOG_LEVEL" env-default:"info"`
}

// Load reads and validates the configuration from the environment, then checks
// invariants the type system can't (e.g. hold must outlive the poll cadence so a
// transfer leg is never flushed before its mate is even polled, §5.1).
func Load() (*Config, error) {
	// Best-effort: load a local .env into the process environment before reading
	// config, mirroring core. A missing file is fine (real env vars are used in
	// production) — warn and continue. Other read errors (e.g. malformed) are
	// surfaced. The structured logger isn't built yet, so warn via stderr.
	switch err := godotenv.Load(".env"); {
	case errors.Is(err, fs.ErrNotExist):
		log.Println("warning: no .env file found, using environment variables only")
	case err != nil:
		return nil, fmt.Errorf("load .env: %w", err)
	}

	cfg := &Config{}
	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("invalid TIMEZONE %q: %w", c.Timezone, err)
	}
	// The hold must exceed the poll interval (plus skew) so a leg never times
	// out before its mate is polled — the core double-counting safeguard (§5.1).
	if c.TransferHoldDuration <= c.PollInterval {
		return fmt.Errorf("TRANSFER_HOLD_DURATION (%s) must exceed POLL_INTERVAL (%s)", c.TransferHoldDuration, c.PollInterval)
	}
	if c.TransferHoldDuration <= c.TransferPairWindow {
		return fmt.Errorf("TRANSFER_HOLD_DURATION (%s) must exceed TRANSFER_PAIR_WINDOW (%s)", c.TransferHoldDuration, c.TransferPairWindow)
	}
	return nil
}

// Location returns the parsed timezone (validated in Load).
func (c *Config) Location() *time.Location {
	loc, _ := time.LoadLocation(c.Timezone)
	return loc
}
