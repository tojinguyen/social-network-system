package http

import (
	"github.com/gin-gonic/gin"
	"social-network-system/pkg/jwtutil"
	"social-network-system/pkg/middleware"
)

// SetupRouter configures routes for the Feed Service.
func SetupRouter(r *gin.Engine, feedHandler *FeedHandler, tokenManager jwtutil.TokenManager) {
	api := r.Group("/api/v1")
	api.Use(middleware.AuthMiddleware(tokenManager))
	{
		api.GET("/feeds", feedHandler.GetFeed)
	}
}
