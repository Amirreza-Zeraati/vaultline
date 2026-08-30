package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Amirreza-Zeraati/vaultline/internal/database"
	"github.com/Amirreza-Zeraati/vaultline/internal/metrics"
	"github.com/Amirreza-Zeraati/vaultline/internal/redis"
)

// HealthHandler serves liveness and readiness probes.
type HealthHandler struct {
	db      *database.DB
	rdb     *redis.Client
	metrics *metrics.Metrics
}

// NewHealthHandler builds the health handler. metrics may be nil, in which case
// readiness simply doesn't publish dependency gauges.
func NewHealthHandler(db *database.DB, rdb *redis.Client, m *metrics.Metrics) *HealthHandler {
	return &HealthHandler{db: db, rdb: rdb, metrics: m}
}

// Live is a liveness probe: is the process up? It checks no dependencies, so a
// slow database never causes the orchestrator to kill a healthy process.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Ready is a readiness probe: can the app serve traffic? It pings every
// critical dependency and returns 503 if any is down, so load balancers stop
// routing until the app is truly ready.
//
// It also mirrors each result into the dependency_up gauge, so the same check
// that gates traffic is what Prometheus alerts on.
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	checks := gin.H{"database": "ok", "redis": "ok"}
	ready := true

	dbUp := h.db.Ping(ctx) == nil
	if !dbUp {
		checks["database"] = "unavailable"
		ready = false
	}

	redisUp := h.rdb.Ping(ctx) == nil
	if !redisUp {
		checks["redis"] = "unavailable"
		ready = false
	}

	if h.metrics != nil {
		h.metrics.SetDependencyUp("postgres", dbUp)
		h.metrics.SetDependencyUp("redis", redisUp)
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, gin.H{"ready": ready, "checks": checks})
}
