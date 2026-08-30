// Package metrics owns the Prometheus collectors. It uses its own registry
// rather than the global default one, so metrics are injected like every other
// dependency and tests can build a throwaway instance.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds every collector the app reports.
type Metrics struct {
	registry *prometheus.Registry

	// RequestsTotal counts finished HTTP requests by method, route, status.
	RequestsTotal *prometheus.CounterVec
	// RequestDuration observes handler latency in seconds.
	RequestDuration *prometheus.HistogramVec
	// RequestsInFlight tracks requests currently being served.
	RequestsInFlight prometheus.Gauge
	// DependencyUp is 1 when a dependency answered its last health check, else 0.
	DependencyUp *prometheus.GaugeVec
}

// New builds the collectors and registers them, along with the standard Go
// runtime and process collectors (goroutines, GC, memory, file descriptors).
func New() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		registry: reg,
		RequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests processed.",
			},
			[]string{"method", "route", "status"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latency in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "route"},
		),
		RequestsInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Number of HTTP requests currently being served.",
			},
		),
		DependencyUp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "dependency_up",
				Help: "Whether a dependency responded to its last health check (1 up, 0 down).",
			},
			[]string{"dependency"},
		),
	}

	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.RequestsTotal,
		m.RequestDuration,
		m.RequestsInFlight,
		m.DependencyUp,
	)

	return m
}

// SetDependencyUp records the result of a dependency health check.
func (m *Metrics) SetDependencyUp(name string, up bool) {
	v := 0.0
	if up {
		v = 1.0
	}
	m.DependencyUp.WithLabelValues(name).Set(v)
}

// Handler returns the /metrics HTTP handler for this registry.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
