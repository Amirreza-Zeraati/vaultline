package handler

// Handlers aggregates all HTTP handlers. Built in main and passed to the router.
type Handlers struct {
	Auth   *AuthHandler
	Health *HealthHandler
}
