// Package migrate applies database migrations using golang-migrate, reading the
// SQL files embedded in the binary. Up() brings the schema to the newest
// version and is safe to call on every startup — it no-ops when already current.
package migrate

import (
	"errors"
	"io/fs"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // postgres driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// Up applies all pending "up" migrations, advancing the database to the latest
// version. It reads migrations from the embedded fs.FS. Passing an already
// up-to-date database is not an error.
func Up(migrationsFS fs.FS, databaseURL string, log *slog.Logger) error {
	src, err := iofs.New(migrationsFS, ".")
	if err != nil {
		return err
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Info("database schema already up to date")
			return nil
		}
		return err
	}
	log.Info("database migrations applied")
	return nil
}
