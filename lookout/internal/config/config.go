package config

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	TelegramAPIID   int    `env:"TELEGRAM_API_ID" env-required:"true"`
	TelegramAPIHash string `env:"TELEGRAM_API_HASH" env-required:"true"`
	SessionFile     string `env:"SESSION_FILE" env-default:"session.json"`
	SourceBot       string `env:"SOURCE_BOT" env-required:"true"`

	AuthMode      string `env:"AUTH_MODE" env-default:"code"`
	TelegramPhone string `env:"TELEGRAM_PHONE" env-default:""`

	PollInterval time.Duration `env:"POLL_INTERVAL" env-default:"60s"`

	TransferPairWindow time.Duration `env:"TRANSFER_PAIR_WINDOW" env-default:"120s"`

	TransferHoldDuration time.Duration `env:"TRANSFER_HOLD_DURATION" env-default:"5m"`

	FinanceAPIURL   string `env:"FINANCE_API_URL" env-required:"true"`
	FinanceAPIToken string `env:"FINANCE_API_TOKEN" env-default:""`

	StateFile string `env:"STATE_FILE" env-default:"lookout-state.json"`

	Timezone string `env:"TIMEZONE" env-default:"Asia/Tashkent"`

	BalanceSendOnStart bool `env:"BALANCE_SEND_ON_START" env-default:"false"`

	LogLevel string `env:"LOG_LEVEL" env-default:"info"`
}

func Load() (*Config, error) {

	if err := godotenv.Load(".env"); err != nil {
		log.Println("warning: no .env file found, using environment variables only")
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

	switch strings.ToLower(c.AuthMode) {
	case "code", "qr":
	default:
		return fmt.Errorf("invalid AUTH_MODE %q: must be \"code\" or \"qr\"", c.AuthMode)
	}

	if c.TransferHoldDuration <= c.PollInterval {
		return fmt.Errorf("TRANSFER_HOLD_DURATION (%s) must exceed POLL_INTERVAL (%s)", c.TransferHoldDuration, c.PollInterval)
	}
	if c.TransferHoldDuration <= c.TransferPairWindow {
		return fmt.Errorf("TRANSFER_HOLD_DURATION (%s) must exceed TRANSFER_PAIR_WINDOW (%s)", c.TransferHoldDuration, c.TransferPairWindow)
	}
	return nil
}

func (c *Config) Location() *time.Location {
	loc, _ := time.LoadLocation(c.Timezone)
	return loc
}
