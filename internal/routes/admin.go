package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/Amirreza-Zeraati/vaultline/internal/middleware"
)

func registerAdmin(api *gin.RouterGroup, d Deps) {
	admin := api.Group("/admin")
	admin.Use(
		middleware.Auth(d.Sessions, d.Config.Session),
		middleware.RequireRole("admin"),
	)

	admin.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": "pong from admin"})
	})
}
