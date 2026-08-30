// Package apperr defines the application's error type. Every layer returns
// *apperr.Error instead of ad-hoc errors, so the HTTP layer can map an error to
// a status code and a JSON body without knowing which package produced it.
//
// An Error carries four things:
//
//	Code    — stable, machine-readable string for API clients ("not_found")
//	Status  — the HTTP status to respond with
//	Message — safe to show the caller; never contains internal detail
//	cause   — the underlying error, logged server-side but never serialized
//
// The split between Message and cause is the point: `apperr.Internal("could not
// load user").Wrap(dbErr)` shows the client a harmless sentence while the real
// database error still reaches the logs.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a stable, machine-readable error identifier. Clients can branch on
// these; they must not change once released.
type Code string

const (
	CodeValidation   Code = "validation_failed"
	CodeInvalidInput Code = "invalid_input"
	CodeUnauthorized Code = "unauthorized"
	CodeForbidden    Code = "forbidden"
	CodeNotFound     Code = "not_found"
	CodeConflict     Code = "conflict"
	CodeTooLarge     Code = "payload_too_large"
	CodeRateLimited  Code = "rate_limited"
	CodeInternal     Code = "internal_error"
	CodeUnavailable  Code = "unavailable"
)

// Error is the application error type.
type Error struct {
	Code    Code
	Status  int
	Message string
	// Fields holds per-field messages for validation failures.
	Fields map[string]string
	// cause is the wrapped underlying error. Logged, never sent to the client.
	cause error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the wrapped cause to errors.Is / errors.As.
func (e *Error) Unwrap() error { return e.cause }

// Is makes errors.Is compare two *Error values by Code, so callers can write
// errors.Is(err, service.ErrEmailTaken) without pointer identity.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return e.Code == t.Code
}

// IsServer reports whether this represents a server-side fault (5xx). The
// response layer uses it to decide what to log.
func (e *Error) IsServer() bool { return e.Status >= http.StatusInternalServerError }

// clone returns a shallow copy so the With/Wrap helpers never mutate a shared
// sentinel value such as service.ErrEmailTaken.
func (e *Error) clone() *Error {
	c := *e
	if e.Fields != nil {
		c.Fields = make(map[string]string, len(e.Fields))
		for k, v := range e.Fields {
			c.Fields[k] = v
		}
	}
	return &c
}

// Wrap attaches an underlying cause and returns a copy.
func (e *Error) Wrap(cause error) *Error {
	c := e.clone()
	c.cause = cause
	return c
}

// WithField adds one field message and returns a copy.
func (e *Error) WithField(name, msg string) *Error {
	c := e.clone()
	if c.Fields == nil {
		c.Fields = make(map[string]string, 1)
	}
	c.Fields[name] = msg
	return c
}

// WithFields merges in field messages and returns a copy.
func (e *Error) WithFields(fields map[string]string) *Error {
	c := e.clone()
	if c.Fields == nil {
		c.Fields = make(map[string]string, len(fields))
	}
	for k, v := range fields {
		c.Fields[k] = v
	}
	return c
}

// New builds an Error explicitly. Prefer the named constructors below.
func New(code Code, status int, message string) *Error {
	return &Error{Code: code, Status: status, Message: message}
}

// Named constructors, one per common failure mode.

func Validation(message string) *Error {
	return New(CodeValidation, http.StatusUnprocessableEntity, message)
}

func InvalidInput(message string) *Error {
	return New(CodeInvalidInput, http.StatusBadRequest, message)
}

func Unauthorized(message string) *Error {
	return New(CodeUnauthorized, http.StatusUnauthorized, message)
}

func Forbidden(message string) *Error {
	return New(CodeForbidden, http.StatusForbidden, message)
}

func NotFound(message string) *Error {
	return New(CodeNotFound, http.StatusNotFound, message)
}

func Conflict(message string) *Error {
	return New(CodeConflict, http.StatusConflict, message)
}

func PayloadTooLarge(message string) *Error {
	return New(CodeTooLarge, http.StatusRequestEntityTooLarge, message)
}

func RateLimited(message string) *Error {
	return New(CodeRateLimited, http.StatusTooManyRequests, message)
}

func Internal(message string) *Error {
	return New(CodeInternal, http.StatusInternalServerError, message)
}

func Unavailable(message string) *Error {
	return New(CodeUnavailable, http.StatusServiceUnavailable, message)
}

// From converts any error into an *Error. An error that already is (or wraps)
// an *Error is returned as-is; anything else becomes a generic 500 with the
// original preserved as the cause, so unexpected errors never leak their text
// to the client.
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal("internal server error").Wrap(err)
}
