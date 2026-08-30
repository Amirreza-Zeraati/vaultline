// Package routes registers the HTTP routes, one file per domain. The server
// package owns the engine and global middleware; this package owns which
// endpoints exist and which guards protect them.
package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/Amirreza-Zeraati/vaultline/internal/config"
	"github.com/Amirreza-Zeraati/vaultline/internal/handler"
	"github.com/Amirreza-Zeraati/vaultline/internal/session"
)

// Deps is everything the route files need to attach handlers and guards.
type Deps struct {
	Config   *config.Config
	Handlers *handler.Handlers
	Sessions session.Store
}

// Register mounts every route group under the given /api/v1 group.
func Register(api *gin.RouterGroup, d Deps) {
	registerAuth(api, d)
	registerAdmin(api, d)
	// registerUser(api, d)  // add as you grow
}
