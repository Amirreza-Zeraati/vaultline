// Package service holds business logic. It sits between the HTTP handlers and
// the repositories: handlers translate HTTP <-> service calls, services own the
// rules, repositories own persistence. Services never import gin or net/http.
package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Amirreza-Zeraati/vaultline/internal/apperr"
	"github.com/Amirreza-Zeraati/vaultline/internal/models"
	"github.com/Amirreza-Zeraati/vaultline/internal/repository"
)

// dummyHash is a valid bcrypt hash of a value nobody will guess. Comparing
// against it when the email is unknown keeps login timing roughly constant, so
// an attacker can't detect registered emails from response latency.
const dummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

// AuthService handles registration and credential verification. It knows
// nothing about sessions or cookies — that's the handler's job.
type AuthService struct {
	users repository.UserRepository
}

// NewAuthService constructs the auth service.
func NewAuthService(users repository.UserRepository) *AuthService {
	return &AuthService{users: users}
}

// Register creates a new user with a bcrypt-hashed password and the default
// "user" role. Returns ErrEmailTaken if the email already exists.
func (s *AuthService) Register(ctx context.Context, email, password string) (*models.User, error) {
	_, err := s.users.GetByEmail(ctx, email)
	switch {
	case err == nil:
		return nil, ErrEmailTaken
	case !errors.Is(err, repository.ErrNotFound):
		return nil, apperr.Internal("could not create account").Wrap(err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperr.Internal("could not create account").Wrap(err)
	}

	user := &models.User{
		Email:        email,
		PasswordHash: string(hash),
		Role:         models.RoleUser,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, apperr.Internal("could not create account").Wrap(err)
	}
	return user, nil
}

// Authenticate verifies an email/password pair and returns the user on success.
// It returns ErrInvalidCredentials for both "no such user" and "wrong password"
// so callers can't distinguish the two (avoids user enumeration).
func (s *AuthService) Authenticate(ctx context.Context, email, password string) (*models.User, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			_ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(password))
			return nil, ErrInvalidCredentials
		}
		return nil, apperr.Internal("could not sign in").Wrap(err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

// GetByID returns a user by ID, mapping the repo's not-found to ErrUserNotFound.
func (s *AuthService) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, apperr.Internal("could not load user").Wrap(err)
	}
	return user, nil
}

// EnsureUser creates a user with the given role, or updates an existing user's
// role (and password, if one is supplied). It's how the admin CLI bootstraps
// the first administrator, since Register always assigns the "user" role.
//
// Returns the user and whether it was newly created.
func (s *AuthService) EnsureUser(ctx context.Context, email, password, role string) (*models.User, bool, error) {
	if !models.IsValidRole(role) {
		return nil, false, apperr.InvalidInput("unknown role: " + role)
	}

	existing, err := s.users.GetByEmail(ctx, email)
	switch {
	case err == nil:
		// Promote (or demote) the existing account.
		existing.Role = role
		if password != "" {
			hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if hashErr != nil {
				return nil, false, apperr.Internal("could not hash password").Wrap(hashErr)
			}
			existing.PasswordHash = string(hash)
		}
		if err := s.users.Update(ctx, existing); err != nil {
			return nil, false, apperr.Internal("could not update user").Wrap(err)
		}
		return existing, false, nil

	case !errors.Is(err, repository.ErrNotFound):
		return nil, false, apperr.Internal("could not look up user").Wrap(err)
	}

	// No such user yet — create one.
	if password == "" {
		return nil, false, apperr.InvalidInput("password is required when creating a new user")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, false, apperr.Internal("could not hash password").Wrap(err)
	}

	user := &models.User{
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, false, apperr.Internal("could not create user").Wrap(err)
	}
	return user, true, nil
}
