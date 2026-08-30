package service

import "github.com/Amirreza-Zeraati/vaultline/internal/apperr"

// Domain-level errors returned by services. Each already carries the HTTP
// status and machine-readable code, so handlers just pass them to
// response.Fail without mapping anything themselves.
//
// These are shared values; apperr's Wrap/WithField helpers copy before
// mutating, so decorating one at a call site cannot corrupt the sentinel.
var (
	ErrInvalidCredentials = apperr.Unauthorized("invalid email or password")
	ErrEmailTaken         = apperr.Conflict("email already registered")
	ErrUserNotFound       = apperr.NotFound("user not found")
)
