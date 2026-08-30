package config

import (
	"testing"
	"time"
)

func TestDatabaseDSN(t *testing.T) {
	db := Database{
		Host: "localhost", Port: 5432, User: "postgres",
		Password: "secret", Name: "app", SSLMode: "disable",
	}

	want := "host=localhost port=5432 user=postgres password=secret dbname=app sslmode=disable"
	if got := db.DSN(); got != want {
		t.Errorf("DSN()\n got: %s\nwant: %s", got, want)
	}
}

func TestDatabaseURL(t *testing.T) {
	db := Database{
		Host: "db.internal", Port: 5432, User: "postgres",
		Password: "secret", Name: "app", SSLMode: "require",
	}

	want := "postgres://postgres:secret@db.internal:5432/app?sslmode=require"
	if got := db.URL(); got != want {
		t.Errorf("URL()\n got: %s\nwant: %s", got, want)
	}
}

func TestRedisAddr(t *testing.T) {
	r := Redis{Host: "cache", Port: 6379}
	if got, want := r.Addr(), "cache:6379"; got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}
}

func TestIsProduction(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"production", true},
		{"development", false},
		{"staging", false},
		{"", false},
		{"Production", false}, // exact match only
	}

	for _, tc := range tests {
		t.Run(tc.env, func(t *testing.T) {
			if got := (App{Env: tc.env}).IsProduction(); got != tc.want {
				t.Errorf("IsProduction() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	// No .env file exists in this package directory, so defaults apply.
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.App.Port != 8080 {
		t.Errorf("App.Port = %d, want 8080", cfg.App.Port)
	}
	if cfg.Session.TTL != 24*time.Hour {
		t.Errorf("Session.TTL = %v, want 24h", cfg.Session.TTL)
	}
	if !cfg.Database.AutoMigrate {
		t.Error("Database.AutoMigrate should default to true")
	}
	if cfg.Metrics.Path != "/metrics" {
		t.Errorf("Metrics.Path = %q, want /metrics", cfg.Metrics.Path)
	}
	if len(cfg.CORS.AllowedMethods) == 0 {
		t.Error("CORS.AllowedMethods should have defaults")
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_PORT", "9090")
	t.Setenv("SESSION_TTL", "15m")
	t.Setenv("RATE_LIMIT_REQUESTS", "42")
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://a.example,https://b.example")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if !cfg.App.IsProduction() {
		t.Error("expected production env")
	}
	if cfg.App.Port != 9090 {
		t.Errorf("App.Port = %d, want 9090", cfg.App.Port)
	}
	if cfg.Session.TTL != 15*time.Minute {
		t.Errorf("Session.TTL = %v, want 15m", cfg.Session.TTL)
	}
	if cfg.RateLimit.Requests != 42 {
		t.Errorf("RateLimit.Requests = %d, want 42", cfg.RateLimit.Requests)
	}
	if len(cfg.CORS.AllowedOrigins) != 2 {
		t.Fatalf("CORS.AllowedOrigins = %v, want 2 entries", cfg.CORS.AllowedOrigins)
	}
	if cfg.CORS.AllowedOrigins[1] != "https://b.example" {
		t.Errorf("second origin = %q", cfg.CORS.AllowedOrigins[1])
	}
}
