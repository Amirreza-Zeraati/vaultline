// Package logger configures the application's structured logger using the
// standard library's log/slog. It returns a *slog.Logger and also installs it
// as the default, so package-level slog.Info/Error calls elsewhere work too.
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New builds a *slog.Logger from a level ("debug"|"info"|"warn"|"error") and a
// format ("json"|"text"). Unknown values fall back to info / text.
func New(level, format string) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: parseLevel(level),
	}

	var handler slog.Handler
	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	l := slog.New(handler)
	slog.SetDefault(l)
	return l
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
