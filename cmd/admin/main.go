// Command admin creates a user with a given role, or promotes an existing one.
// It exists because the public /auth/register endpoint always assigns the
// "user" role — there has to be some out-of-band way to mint the first admin.
//
//	go run ./cmd/admin -email=me@example.com -password='s3cret' -role=admin
//	docker compose run --rm app /app/admin -email=me@example.com -password='s3cret'
//
// It reuses the same config, database, and service layer as the API, so
// password hashing and validation behave identically to a real registration.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Amirreza-Zeraati/vaultline/internal/config"
	"github.com/Amirreza-Zeraati/vaultline/internal/database"
	"github.com/Amirreza-Zeraati/vaultline/internal/models"
	"github.com/Amirreza-Zeraati/vaultline/internal/repository"
	"github.com/Amirreza-Zeraati/vaultline/internal/service"
	"github.com/Amirreza-Zeraati/vaultline/pkg/logger"
)

func main() {
	if err := run(); err != nil {
		slog.Error("admin command failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		email    = flag.String("email", "", "email address of the user (required)")
		password = flag.String("password", "", "password; required when creating, optional when promoting")
		role     = flag.String("role", models.RoleAdmin, "role to assign: user | admin")
	)
	flag.Parse()

	if *email == "" {
		flag.Usage()
		return fmt.Errorf("-email is required")
	}
	if !models.IsValidRole(*role) {
		return fmt.Errorf("invalid -role %q (valid: %v)", *role, models.ValidRoles)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := logger.New(cfg.Log.Level, cfg.Log.Format)

	db, err := database.New(cfg.Database, cfg.App.IsProduction())
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.WaitForConnection(ctx, log); err != nil {
		return err
	}

	repos := &repository.Repositories{User: repository.NewUserRepository(db)}
	svc := service.NewAuthService(repos.User)

	user, created, err := svc.EnsureUser(ctx, *email, *password, *role)
	if err != nil {
		return err
	}

	action := "updated"
	if created {
		action = "created"
	}
	log.Info("user "+action, "email", user.Email, "role", user.Role, "id", user.ID.String())
	return nil
}
