package config

import (
	"fmt"
	"net"
	"net/url"

	"github.com/ilyakaznacheev/cleanenv"
	"go.uber.org/fx"
	"go.uber.org/zap/zapcore"
)

type (
	Config struct {
		App
		HTTP
		Log
		DB
		Finance
		Ingest
		Auth
		Anthropic
		Push
	}

	App struct {
		Name string `env:"APP_NAME" env-required:"true"`
	}

	HTTP struct {
		Port       string `env:"HTTP_PORT"    env-required:"true"`
		CORSOrigin string `env:"CORS_ORIGIN"  env-default:""`
	}

	Log struct {
		Level        string `env:"LOG_LEVEL" env-required:"true"`
		SamplingRate int    `env:"LOG_SAMPLING_RATE" env-default:"100"`
	}

	DB struct {
		Host           string `env:"POSTGRES_HOST"      env-required:"true"`
		Port           string `env:"POSTGRES_PORT"      env-required:"true"`
		User           string `env:"POSTGRES_USER"      env-required:"true"`
		Password       string `env:"POSTGRES_PASSWORD"  env-required:"true"`
		Name           string `env:"POSTGRES_DB"        env-required:"true"`
		SSLMode        string `env:"POSTGRES_SSLMODE"   env-default:"disable"`
		MigrationsPath string `env:"DB_MIGRATIONS_PATH" env-required:"true"`
	}

	Finance struct {
		BaseCurrency string `env:"BASE_CURRENCY" env-default:"UZS"`
	}

	Ingest struct {
		Token string `env:"INGEST_TOKEN" env-default:""`
	}

	Auth struct {
		Username string `env:"AUTH_USERNAME" env-required:"true"`
		Password string `env:"AUTH_PASSWORD" env-required:"true"`
	}

	Anthropic struct {
		APIKey string `env:"ANTHROPIC_API_KEY" env-default:""`
		Model  string `env:"ANTHROPIC_MODEL"   env-default:"claude-haiku-4-5"`
	}

	// Push holds Web Push VAPID credentials. Empty keys disable push entirely.
	Push struct {
		VAPIDPublic  string `env:"VAPID_PUBLIC_KEY"  env-default:""`
		VAPIDPrivate string `env:"VAPID_PRIVATE_KEY" env-default:""`
		Subscriber   string `env:"VAPID_SUBSCRIBER"  env-default:""` // mailto: or https contact for push services
	}
)

func NewConfig() (*Config, error) {
	cfg := &Config{}
	if err := cleanenv.ReadEnv(cfg); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	return cfg, nil
}

func RegisterConfigs() fx.Option {
	return fx.Provide(
		NewConfig,
		func(cfg *Config) *App { return &cfg.App },
		func(cfg *Config) *HTTP { return &cfg.HTTP },
		func(cfg *Config) *Log { return &cfg.Log },
		func(cfg *Config) *DB { return &cfg.DB },
		func(cfg *Config) *Finance { return &cfg.Finance },
		func(cfg *Config) *Ingest { return &cfg.Ingest },
		func(cfg *Config) *Auth { return &cfg.Auth },
		func(cfg *Config) *Anthropic { return &cfg.Anthropic },
		func(cfg *Config) *Push { return &cfg.Push },
	)
}

// DSN builds a PostgreSQL connection string for the pgx pool.
func (d *DB) DSN() string {
	return d.dsn("postgres")
}

// MigrationDSN builds the connection string for golang-migrate. Its pgx/v5
// database driver registers under the "pgx5" URL scheme.
func (d *DB) MigrationDSN() string {
	return d.dsn("pgx5")
}

func (d *DB) dsn(scheme string) string {
	u := url.URL{
		Scheme:   scheme,
		User:     url.UserPassword(d.User, d.Password),
		Host:     net.JoinHostPort(d.Host, d.Port),
		Path:     d.Name,
		RawQuery: url.Values{"sslmode": {d.SSLMode}}.Encode(),
	}

	return u.String()
}

func (l *Log) ZapLevel() zapcore.Level {
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(l.Level)); err != nil {
		panic(fmt.Sprintf("invalid log level: %s", l.Level))
	}

	return level
}
