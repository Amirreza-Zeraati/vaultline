package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Amirreza-Zeraati/vaultline/internal/config"
)

// Server wraps http.Server with typed config and a logger.
type Server struct {
	httpServer *http.Server
	log        *slog.Logger
}

// New builds the HTTP server from config and a router.
func New(cfg *config.Config, router *gin.Engine) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.App.Port),
			Handler: router,
			// ReadHeaderTimeout is the slowloris guard; ReadTimeout bounds the
			// whole request, WriteTimeout the response.
			ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
			ReadTimeout:       cfg.HTTP.ReadTimeout,
			WriteTimeout:      cfg.HTTP.WriteTimeout,
			IdleTimeout:       cfg.HTTP.IdleTimeout,
			MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
		},
	}
}

// Start begins listening. It blocks until the server stops. A clean shutdown
// returns nil rather than http.ErrServerClosed.
func (s *Server) Start(log *slog.Logger) error {
	s.log = log
	log.Info("http server listening", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully drains in-flight requests until the context deadline.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.log != nil {
		s.log.Info("http server shutting down")
	}
	return s.httpServer.Shutdown(ctx)
}
