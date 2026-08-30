// Package handler contains the Gin HTTP handlers. Handlers translate between
// HTTP and the service layer: bind+validate input, call a service, and write a
// response. Error-to-status mapping lives in the error itself (see apperr), so
// handlers just forward failures to response.Fail. No business logic here.
package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Amirreza-Zeraati/vaultline/internal/apperr"
	"github.com/Amirreza-Zeraati/vaultline/internal/config"
	"github.com/Amirreza-Zeraati/vaultline/internal/dto"
	"github.com/Amirreza-Zeraati/vaultline/internal/middleware"
	"github.com/Amirreza-Zeraati/vaultline/internal/models"
	"github.com/Amirreza-Zeraati/vaultline/internal/response"
	"github.com/Amirreza-Zeraati/vaultline/internal/session"
)

// AuthService is the behavior AuthHandler needs. Declaring the interface here,
// on the consumer side, is the idiomatic Go pattern: the service package stays
// free of interfaces it doesn't use, and tests can pass a fake.
// *service.AuthService satisfies this.
type AuthService interface {
	Register(ctx context.Context, email, password string) (*models.User, error)
	Authenticate(ctx context.Context, email, password string) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

// AuthHandler serves the auth endpoints.
type AuthHandler struct {
	auth     AuthService
	sessions session.Store
	cfg      config.Session
}

// NewAuthHandler builds the auth handler.
func NewAuthHandler(auth AuthService, sessions session.Store, cfg config.Session) *AuthHandler {
	return &AuthHandler{auth: auth, sessions: sessions, cfg: cfg}
}

// Register handles POST /auth/register.
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.BindError(err))
		return
	}

	user, err := h.auth.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.JSON(c, http.StatusCreated, dto.NewUserResponse(user))
}

// Login handles POST /auth/login: verify credentials, create a session, set
// the cookie.
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, response.BindError(err))
		return
	}

	user, err := h.auth.Authenticate(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		response.Fail(c, err)
		return
	}

	sid, err := h.sessions.Create(c.Request.Context(), session.Session{
		UserID: user.ID,
		Role:   user.Role,
	})
	if err != nil {
		response.Fail(c, apperr.Internal("could not create session").Wrap(err))
		return
	}

	middleware.SetSessionCookie(c, h.cfg, sid, int(h.cfg.TTL.Seconds()))
	response.JSON(c, http.StatusOK, dto.NewUserResponse(user))
}

// Logout handles POST /auth/logout: destroy the session and clear the cookie.
// Requires the Auth middleware to have run.
func (h *AuthHandler) Logout(c *gin.Context) {
	if sid, ok := middleware.CurrentSessionID(c); ok {
		_ = h.sessions.Delete(c.Request.Context(), sid)
	}
	middleware.ClearSessionCookie(c, h.cfg)
	response.JSON(c, http.StatusOK, gin.H{"message": "logged out"})
}

// Me handles GET /auth/me: return the current user. Requires Auth middleware.
func (h *AuthHandler) Me(c *gin.Context) {
	id, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Fail(c, apperr.Unauthorized("authentication required"))
		return
	}

	user, err := h.auth.GetByID(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.JSON(c, http.StatusOK, dto.NewUserResponse(user))
}
