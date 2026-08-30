package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/Amirreza-Zeraati/vaultline/internal/apperr"
	"github.com/Amirreza-Zeraati/vaultline/internal/config"
	"github.com/Amirreza-Zeraati/vaultline/internal/response"
	"github.com/Amirreza-Zeraati/vaultline/internal/session"
)

// Auth requires a valid session. It reads the session cookie, looks the session
// up in Redis, refreshes its TTL (sliding expiration), and stores the user ID,
// role, and session ID in the request context. No database hit is needed —
// the role is cached in the session.
func Auth(store session.Store, cfg config.Session) gin.HandlerFunc {
	return func(c *gin.Context) {
		sid, err := c.Cookie(cfg.CookieName)
		if err != nil || sid == "" {
			response.AbortFail(c, apperr.Unauthorized("authentication required"))
			return
		}

		sess, err := store.Get(c.Request.Context(), sid)
		if err != nil {
			if errors.Is(err, session.ErrNotFound) {
				ClearSessionCookie(c, cfg)
				response.AbortFail(c, apperr.Unauthorized("session expired"))
				return
			}
			response.AbortFail(c, apperr.Internal("could not verify session").Wrap(err))
			return
		}

		// Sliding expiration; ignore the error — worst case it expires on schedule.
		_ = store.Refresh(c.Request.Context(), sid)

		c.Set(ctxUserID, sess.UserID)
		c.Set(ctxRole, sess.Role)
		c.Set(ctxSessionID, sid)
		c.Next()
	}
}

// RequireRole authorizes only the listed roles. Must run after Auth. Returns
// 403 for an authenticated user whose role isn't allowed.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		role, ok := CurrentRole(c)
		if !ok {
			response.AbortFail(c, apperr.Unauthorized("authentication required"))
			return
		}
		if _, ok := allowed[role]; !ok {
			response.AbortFail(c, apperr.Forbidden("insufficient permissions"))
			return
		}
		c.Next()
	}
}
