package http

import (
	"github.com/gin-gonic/gin"
	"social-network-system/pkg/jwtutil"
	"social-network-system/pkg/middleware"
)

// SetupRouter configures routes for the Auth Service.
func SetupRouter(r *gin.Engine, handler *AuthHandler, tokenManager jwtutil.TokenManager) {
	api := r.Group("/api/v1/auth")
	{
		api.POST("/register", handler.Register)
		api.POST("/login", handler.Login)
		api.POST("/refresh", handler.Refresh)
		api.POST("/logout", handler.Logout)

		// Protected endpoint
		api.GET("/me", middleware.AuthMiddleware(tokenManager), handler.Me)
	}
}
