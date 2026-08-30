package middleware

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Amirreza-Zeraati/vaultline/internal/apperr"
	"github.com/Amirreza-Zeraati/vaultline/internal/config"
	"github.com/Amirreza-Zeraati/vaultline/internal/redis"
	"github.com/Amirreza-Zeraati/vaultline/internal/response"
)

// RateLimit applies a fixed-window limit per client IP, backed by Redis so the
// limit is shared across all app instances. INCR and the expiry are pipelined
// into a single round-trip.
//
// The counter and its TTL are set by a small Lua script (see
// redis.FixedWindowIncr) that attaches the expiry only when it creates the key.
// A plain EXPIRE on every request — the obvious implementation — keeps pushing
// the deadline out, so a client sending continuous traffic never gets a fresh
// window and stays blocked indefinitely rather than for one window.
func RateLimit(rdb *redis.Client, cfg config.RateLimit) gin.HandlerFunc {
	windowSecs := strconv.Itoa(int(cfg.Window.Seconds()))
	limitStr := strconv.Itoa(cfg.Requests)

	return func(c *gin.Context) {
		if !cfg.Enabled {
			c.Next()
			return
		}

		key := "ratelimit:" + c.ClientIP()

		ctx, cancel := context.WithTimeout(c.Request.Context(), 200*time.Millisecond)
		defer cancel()

		count, err := rdb.FixedWindowIncr(ctx, key, cfg.Window)
		if err != nil {
			// Fail open: if Redis is briefly unavailable, don't lock users out.
			c.Next()
			return
		}

		remaining := int64(cfg.Requests) - count
		if remaining < 0 {
			remaining = 0
		}

		c.Header("X-RateLimit-Limit", limitStr)
		c.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

		if count > int64(cfg.Requests) {
			c.Header("Retry-After", windowSecs)
			response.AbortFail(c, apperr.RateLimited("rate limit exceeded"))
			return
		}
		c.Next()
	}
}
