package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Amirreza-Zeraati/vaultline/internal/metrics"
)

// Metrics records request count, latency, and in-flight count.
//
// The "route" label uses c.FullPath() — the registered pattern such as
// "/api/v1/users/:id" — not the raw URL. Using the raw URL would create a new
// time series per user ID and blow up Prometheus cardinality. Requests that
// match no route report "unmatched" for the same reason.
func Metrics(m *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		m.RequestsInFlight.Inc()

		c.Next()

		m.RequestsInFlight.Dec()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		method := c.Request.Method
		status := strconv.Itoa(c.Writer.Status())

		m.RequestsTotal.WithLabelValues(method, route, status).Inc()
		m.RequestDuration.WithLabelValues(method, route).Observe(time.Since(start).Seconds())
	}
}
