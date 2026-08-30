// Package config loads and validates application configuration from the
// environment. Everything else in the app reads config from here — nothing
// else should call os.Getenv directly.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is the fully-parsed application configuration.
type Config struct {
	App       App
	HTTP      HTTP
	Database  Database
	Redis     Redis
	Session   Session
	CORS      CORS
	RateLimit RateLimit
	Metrics   Metrics
	Log       Log
}

type App struct {
	// Env is one of: development, staging, production.
	Env  string `env:"APP_ENV" envDefault:"development"`
	Port int    `env:"APP_PORT" envDefault:"8080"`

	// TrustedProxies lists CIDRs or IPs of reverse proxies allowed to set
	// X-Forwarded-For. Empty means trust nothing: the client IP is the direct
	// peer address. Leaving this empty while running behind a proxy makes every
	// client look like the proxy; filling it in while NOT behind one lets
	// clients spoof their IP and slip past the rate limiter.
	TrustedProxies []string `env:"TRUSTED_PROXIES" envSeparator:"," envDefault:""`
}

type HTTP struct {
	// ReadHeaderTimeout bounds how long a client may take to send its headers.
	// Without it, a slowloris client can hold a connection open indefinitely by
	// dribbling out one header byte at a time.
	ReadHeaderTimeout time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" envDefault:"5s"`
	// MaxHeaderBytes caps total header size.
	MaxHeaderBytes int `env:"HTTP_MAX_HEADER_BYTES" envDefault:"1048576"`
	// MaxBodyBytes caps request body size (1 MiB default).
	MaxBodyBytes int64 `env:"HTTP_MAX_BODY_BYTES" envDefault:"1048576"`

	ReadTimeout     time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"10s"`
	WriteTimeout    time.Duration `env:"HTTP_WRITE_TIMEOUT" envDefault:"10s"`
	IdleTimeout     time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"15s"`
}

type Database struct {
	Host     string `env:"DB_HOST" envDefault:"localhost"`
	Port     int    `env:"DB_PORT" envDefault:"5432"`
	User     string `env:"DB_USER" envDefault:"postgres"`
	Password string `env:"DB_PASSWORD" envDefault:"postgres"`
	Name     string `env:"DB_NAME" envDefault:"app"`
	SSLMode  string `env:"DB_SSLMODE" envDefault:"disable"`

	// AutoMigrate runs pending migrations on startup when true.
	AutoMigrate bool `env:"DB_AUTO_MIGRATE" envDefault:"true"`

	MaxOpenConns    int           `env:"DB_MAX_OPEN_CONNS" envDefault:"25"`
	MaxIdleConns    int           `env:"DB_MAX_IDLE_CONNS" envDefault:"25"`
	ConnMaxLifetime time.Duration `env:"DB_CONN_MAX_LIFETIME" envDefault:"5m"`
	// ConnMaxIdleTime returns connections idle longer than this to the OS, so a
	// traffic spike doesn't leave the pool holding open sockets forever.
	ConnMaxIdleTime time.Duration `env:"DB_CONN_MAX_IDLE_TIME" envDefault:"5m"`
}

type Redis struct {
	Host     string `env:"REDIS_HOST" envDefault:"localhost"`
	Port     int    `env:"REDIS_PORT" envDefault:"6379"`
	Password string `env:"REDIS_PASSWORD" envDefault:""`
	DB       int    `env:"REDIS_DB" envDefault:"0"`
}

type Session struct {
	CookieName   string        `env:"SESSION_COOKIE_NAME" envDefault:"session_id"`
	TTL          time.Duration `env:"SESSION_TTL" envDefault:"24h"`
	CookieSecure bool          `env:"SESSION_COOKIE_SECURE" envDefault:"false"`
	CookieDomain string        `env:"SESSION_COOKIE_DOMAIN" envDefault:""`
	CookiePath   string        `env:"SESSION_COOKIE_PATH" envDefault:"/"`
	// CookieSameSite: lax | strict | none
	CookieSameSite string `env:"SESSION_COOKIE_SAMESITE" envDefault:"lax"`
}

type CORS struct {
	AllowedOrigins   []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000"`
	AllowedMethods   []string `env:"CORS_ALLOWED_METHODS" envSeparator:"," envDefault:"GET,POST,PUT,PATCH,DELETE,OPTIONS"`
	AllowedHeaders   []string `env:"CORS_ALLOWED_HEADERS" envSeparator:"," envDefault:"Origin,Content-Type,Accept,Authorization"`
	AllowCredentials bool     `env:"CORS_ALLOW_CREDENTIALS" envDefault:"true"`
	MaxAge           int      `env:"CORS_MAX_AGE" envDefault:"300"`
}

type RateLimit struct {
	Enabled  bool          `env:"RATE_LIMIT_ENABLED" envDefault:"true"`
	Requests int           `env:"RATE_LIMIT_REQUESTS" envDefault:"100"`
	Window   time.Duration `env:"RATE_LIMIT_WINDOW" envDefault:"1m"`
}

type Metrics struct {
	Enabled bool   `env:"METRICS_ENABLED" envDefault:"true"`
	Path    string `env:"METRICS_PATH" envDefault:"/metrics"`
}

type Log struct {
	Level  string `env:"LOG_LEVEL" envDefault:"info"`
	Format string `env:"LOG_FORMAT" envDefault:"text"`
}

// DSN returns the key=value PostgreSQL connection string used by GORM.
func (d Database) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

// URL returns the postgres:// URL form used by golang-migrate.
func (d Database) URL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

// Addr returns the host:port for the Redis client.
func (r Redis) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// IsProduction reports whether the app is running in production mode.
func (a App) IsProduction() bool { return a.Env == "production" }

// Load reads the .env file (if present) and parses environment variables.
// A missing .env file is not an error — production config usually comes from
// real environment variables.
func Load() (*Config, error) {
	_ = godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}
