// Package middleware holds the Gin middleware chain and the context helpers for
// reading the authenticated user set by the auth middleware.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Context keys. Kept unexported and accessed via the helpers below so handlers
// don't hardcode string keys.
const (
	ctxUserID    = "auth.user_id"
	ctxRole      = "auth.role"
	ctxSessionID = "auth.session_id"
	ctxRequestID = "request.id"
)

// CurrentUserID returns the authenticated user's ID, or false if unauthenticated.
func CurrentUserID(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(ctxUserID)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

// CurrentRole returns the authenticated user's role.
func CurrentRole(c *gin.Context) (string, bool) {
	v, ok := c.Get(ctxRole)
	if !ok {
		return "", false
	}
	role, ok := v.(string)
	return role, ok
}

// CurrentSessionID returns the active session ID (needed for logout).
func CurrentSessionID(c *gin.Context) (string, bool) {
	v, ok := c.Get(ctxSessionID)
	if !ok {
		return "", false
	}
	id, ok := v.(string)
	return id, ok
}

// CurrentRequestID returns the per-request correlation ID set by the RequestID
// middleware, or "" if that middleware didn't run.
//
// Named Current* rather than RequestID to avoid colliding with the RequestID()
// middleware constructor in this same package.
func CurrentRequestID(c *gin.Context) string {
	v, ok := c.Get(ctxRequestID)
	if !ok {
		return ""
	}
	id, _ := v.(string)
	return id
}
