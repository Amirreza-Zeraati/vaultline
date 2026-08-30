package apperr

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestConstructorsMapToStatusAndCode(t *testing.T) {
	tests := []struct {
		name       string
		err        *Error
		wantStatus int
		wantCode   Code
	}{
		{"validation", Validation("bad"), http.StatusUnprocessableEntity, CodeValidation},
		{"invalid input", InvalidInput("bad"), http.StatusBadRequest, CodeInvalidInput},
		{"unauthorized", Unauthorized("nope"), http.StatusUnauthorized, CodeUnauthorized},
		{"forbidden", Forbidden("nope"), http.StatusForbidden, CodeForbidden},
		{"not found", NotFound("gone"), http.StatusNotFound, CodeNotFound},
		{"conflict", Conflict("dup"), http.StatusConflict, CodeConflict},
		{"rate limited", RateLimited("slow down"), http.StatusTooManyRequests, CodeRateLimited},
		{"internal", Internal("oops"), http.StatusInternalServerError, CodeInternal},
		{"unavailable", Unavailable("later"), http.StatusServiceUnavailable, CodeUnavailable},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Status != tc.wantStatus {
				t.Errorf("status = %d, want %d", tc.err.Status, tc.wantStatus)
			}
			if tc.err.Code != tc.wantCode {
				t.Errorf("code = %q, want %q", tc.err.Code, tc.wantCode)
			}
		})
	}
}

func TestIsServer(t *testing.T) {
	if NotFound("x").IsServer() {
		t.Error("404 should not count as a server fault")
	}
	if !Internal("x").IsServer() {
		t.Error("500 should count as a server fault")
	}
}

func TestWrapPreservesCause(t *testing.T) {
	cause := errors.New("connection refused")
	err := Internal("could not load user").Wrap(cause)

	if !errors.Is(err, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("Error() should include the cause, got %q", err.Error())
	}
	if err.Message == cause.Error() {
		t.Error("client message must not be replaced by the cause")
	}
}

// Wrap and WithField must copy, so decorating a shared sentinel at one call
// site cannot corrupt it for every other caller.
func TestWrapAndWithFieldDoNotMutateOriginal(t *testing.T) {
	sentinel := Conflict("email already registered")

	wrapped := sentinel.Wrap(errors.New("boom"))
	if sentinel.Unwrap() != nil {
		t.Error("Wrap mutated the original sentinel")
	}
	if wrapped.Unwrap() == nil {
		t.Error("Wrap did not set the cause on the copy")
	}

	withField := sentinel.WithField("email", "is taken")
	if len(sentinel.Fields) != 0 {
		t.Error("WithField mutated the original sentinel")
	}
	if withField.Fields["email"] != "is taken" {
		t.Error("WithField did not set the field on the copy")
	}

	// Copies must not share the same map either.
	second := withField.WithField("password", "too short")
	if _, leaked := withField.Fields["password"]; leaked {
		t.Error("copies share the same Fields map")
	}
	if len(second.Fields) != 2 {
		t.Errorf("second copy has %d fields, want 2", len(second.Fields))
	}
}

func TestIsComparesByCode(t *testing.T) {
	notFoundA := NotFound("user not found")
	notFoundB := NotFound("order not found")
	conflict := Conflict("duplicate")

	if !errors.Is(notFoundA, notFoundB) {
		t.Error("two not-found errors should match by code")
	}
	if errors.Is(notFoundA, conflict) {
		t.Error("different codes should not match")
	}

	// Matching must survive wrapping.
	wrapped := NotFound("user not found").Wrap(errors.New("sql: no rows"))
	if !errors.Is(wrapped, notFoundB) {
		t.Error("wrapped error should still match by code")
	}
}

func TestFrom(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		if From(nil) != nil {
			t.Error("From(nil) should be nil")
		}
	})

	t.Run("passes through an existing apperr", func(t *testing.T) {
		original := NotFound("user not found")
		if got := From(original); got != original {
			t.Error("From should return the same *Error it was given")
		}
	})

	t.Run("finds an apperr wrapped in a plain error", func(t *testing.T) {
		wrapped := errors.Join(errors.New("context"), Forbidden("nope"))
		got := From(wrapped)
		if got.Code != CodeForbidden {
			t.Errorf("code = %q, want %q", got.Code, CodeForbidden)
		}
	})

	t.Run("converts an unknown error to a safe 500", func(t *testing.T) {
		raw := errors.New("pq: password authentication failed for user \"admin\"")
		got := From(raw)

		if got.Status != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", got.Status)
		}
		if strings.Contains(got.Message, "password authentication failed") {
			t.Error("raw error text leaked into the client-facing message")
		}
		if !errors.Is(got, raw) {
			t.Error("original error should be preserved as the cause for logging")
		}
	})
}
