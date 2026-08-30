package service

import "github.com/Amirreza-Zeraati/vaultline/internal/repository"

// Services aggregates every service. Build once in main and pass to handlers.
type Services struct {
	Auth *AuthService
}

// New wires the service layer from the repositories.
func New(repos *repository.Repositories) *Services {
	return &Services{
		Auth: NewAuthService(repos.User),
	}
}
