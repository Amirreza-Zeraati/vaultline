// Package dto holds request/response shapes for the HTTP layer. Keeping them
// separate from models means the wire format can change without touching the
// database schema, and validation tags live in one obvious place.
package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/Amirreza-Zeraati/vaultline/internal/models"
)

// RegisterRequest is the body for POST /auth/register.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"` // bcrypt caps at 72 bytes
}

// LoginRequest is the body for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// UserResponse is the public view of a user (never exposes the password hash).
type UserResponse struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// NewUserResponse maps a model to its public representation.
func NewUserResponse(u *models.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
	}
}
