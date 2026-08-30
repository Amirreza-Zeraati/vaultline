package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/Amirreza-Zeraati/vaultline/internal/middleware"
)

func registerAuth(api *gin.RouterGroup, d Deps) {
	auth := api.Group("/auth")

	// public
	auth.POST("/register", d.Handlers.Auth.Register)
	auth.POST("/login", d.Handlers.Auth.Login)

	// session-protected
	authed := auth.Group("")
	authed.Use(middleware.Auth(d.Sessions, d.Config.Session))
	authed.POST("/logout", d.Handlers.Auth.Logout)
	authed.GET("/me", d.Handlers.Auth.Me)
}
