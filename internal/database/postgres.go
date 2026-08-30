// Package database owns the PostgreSQL connection lifecycle: opening a pooled
// *gorm.DB, applying pool limits, and exposing Ping (for readiness checks) and
// Close (for graceful shutdown).
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Amirreza-Zeraati/vaultline/internal/config"
)

// DB wraps *gorm.DB so the rest of the app depends on this package rather than
// GORM directly. Swapping the ORM later touches only this file.
type DB struct {
	*gorm.DB
}

// New opens a connection to PostgreSQL and configures the underlying pool.
// It does NOT ping — call Ping separately so the caller controls the timeout.
func New(cfg config.Database, appIsProd bool) (*DB, error) {
	gormLogLevel := gormLoggerLevel(appIsProd)

	gdb, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger:                 logger.Default.LogMode(gormLogLevel),
		SkipDefaultTransaction: true, // we manage transactions explicitly
		PrepareStmt:            true, // cache prepared statements
	})
	if err != nil {
		return nil, fmt.Errorf("open gorm: %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	return &DB{gdb}, nil
}

// Ping verifies the connection is alive, honoring the context deadline. Use
// this in readiness endpoints.
func (db *DB) Ping(ctx context.Context) error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// Close releases the pool. Call during graceful shutdown.
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// WaitForConnection pings repeatedly until success or the context is done.
// Useful on startup when the DB container may not be ready yet.
func (db *DB) WaitForConnection(ctx context.Context, log *slog.Logger) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		if err := db.Ping(ctx); err == nil {
			return nil
		}
		log.Info("waiting for database...")
		select {
		case <-ctx.Done():
			return fmt.Errorf("database not reachable before timeout: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func gormLoggerLevel(isProd bool) logger.LogLevel {
	if isProd {
		return logger.Error // only log DB errors in prod
	}
	return logger.Info // log all SQL in dev
}
