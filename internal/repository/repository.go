// Package repository defines the data-access layer. Interfaces live here so the
// service layer depends on behavior, not on GORM. Concrete GORM implementations
// live alongside (e.g. user_repository.go). This makes services trivial to
// unit-test with a mock repo.
package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/Amirreza-Zeraati/vaultline/internal/models"
)

// ErrNotFound is returned when a lookup finds no matching row. Callers check
// this instead of leaking gorm.ErrRecordNotFound up the stack.
var ErrNotFound = errors.New("record not found")

// UserRepository is the contract for user persistence.
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int) ([]models.User, error)
}

// Repositories aggregates every repository. Wire it once in main and pass it
// into the service layer.
type Repositories struct {
	User UserRepository
}
