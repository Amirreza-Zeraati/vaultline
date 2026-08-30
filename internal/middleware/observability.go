package middleware

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Amirreza-Zeraati/vaultline/internal/apperr"
	"github.com/Amirreza-Zeraati/vaultline/internal/response"
)

const requestIDHeader = "X-Request-ID"

// RequestID assigns each request a correlation ID (reusing an inbound
// X-Request-ID if present) and echoes it back in the response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(ctxRequestID, id)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}

// Logger logs one structured line per request after it completes.
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
			"request_id", CurrentRequestID(c),
		}
		switch {
		case c.Writer.Status() >= 500:
			log.Error("request", attrs...)
		case c.Writer.Status() >= 400:
			log.Warn("request", attrs...)
		default:
			log.Info("request", attrs...)
		}
	}
}

// Recovery converts panics into a clean JSON 500 (instead of gin's default
// plaintext) and logs the panic with its request ID.
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered",
					"error", r,
					"path", c.Request.URL.Path,
					"request_id", CurrentRequestID(c),
				)
				// Wrap the panic value so the cause reaches the logs while the
				// client only sees a generic message.
				err := apperr.Internal("internal server error").
					Wrap(fmt.Errorf("panic: %v", r))
				response.AbortFail(c, err)
			}
		}()
		c.Next()
	}
}
