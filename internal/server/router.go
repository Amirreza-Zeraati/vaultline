// Package server builds the Gin router and runs the HTTP server with graceful
// shutdown.
package server

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/Amirreza-Zeraati/vaultline/internal/config"
	"github.com/Amirreza-Zeraati/vaultline/internal/handler"
	"github.com/Amirreza-Zeraati/vaultline/internal/metrics"
	"github.com/Amirreza-Zeraati/vaultline/internal/middleware"
	"github.com/Amirreza-Zeraati/vaultline/internal/redis"
	"github.com/Amirreza-Zeraati/vaultline/internal/routes"
	"github.com/Amirreza-Zeraati/vaultline/internal/session"
)

// Deps are everything the router needs to wire routes and middleware.
type Deps struct {
	Config   *config.Config
	Log      *slog.Logger
	Redis    *redis.Client
	Sessions session.Store
	Handlers *handler.Handlers
	Metrics  *metrics.Metrics
}

// NewRouter builds the *gin.Engine with the global middleware chain and routes.
func NewRouter(d Deps) *gin.Engine {
	if d.Config.App.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// New() not Default() so we control the middleware order explicitly.
	r := gin.New()

	// Gin trusts ALL proxies by default, which means any client can set
	// X-Forwarded-For and control what ClientIP() returns — spoofing the value
	// the rate limiter keys on and the value written to the logs. Trust only
	// the configured proxies; an empty list trusts none.
	if err := r.SetTrustedProxies(d.Config.App.TrustedProxies); err != nil {
		d.Log.Error("invalid TRUSTED_PROXIES, falling back to trusting none", "err", err)
		_ = r.SetTrustedProxies(nil)
	}

	// Global chain — order matters: request ID first (so logs/panics have it),
	// then recovery, logging, metrics, CORS.
	//
	// Rate limiting is deliberately NOT global: it's applied to /api/v1 below,
	// so health probes and metrics scrapes are never throttled.
	r.Use(
		middleware.RequestID(),
		middleware.Recovery(d.Log),
		middleware.Logger(d.Log),
		middleware.BodyLimit(d.Config.HTTP.MaxBodyBytes),
	)
	if d.Metrics != nil {
		r.Use(middleware.Metrics(d.Metrics))
	}
	r.Use(middleware.CORS(d.Config.CORS))

	// Health probes: no auth, no rate limit.
	r.GET("/healthz", d.Handlers.Health.Live)
	r.GET("/readyz", d.Handlers.Health.Ready)

	// Prometheus scrape endpoint.
	if d.Config.Metrics.Enabled && d.Metrics != nil {
		r.GET(d.Config.Metrics.Path, gin.WrapH(d.Metrics.Handler()))
	}

	api := r.Group("/api/v1")
	api.Use(middleware.RateLimit(d.Redis, d.Config.RateLimit))

	routes.Register(api, routes.Deps{
		Config:   d.Config,
		Handlers: d.Handlers,
		Sessions: d.Sessions,
	})

	return r
}
