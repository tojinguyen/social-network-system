package http

import (
	"github.com/gin-gonic/gin"
	"social-network-system/pkg/jwtutil"
	"social-network-system/pkg/middleware"
	"social-network-system/services/chat/internal/delivery/ws"
)

// SetupRouter registers all HTTP and WebSocket endpoints for Chat Service.
func SetupRouter(
	r *gin.Engine,
	chatHandler *ChatHandler,
	wsHandler *ws.WSHandler,
	tokenManager jwtutil.TokenManager,
) {
	// Root endpoint for API Gateway routing check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP", "node_id": wsHandler.NodeID()})
	})

	api := r.Group("/api/v1/chats")
	{
		// HTTP API to get chat history (protected by auth middleware)
		api.GET("", middleware.AuthMiddleware(tokenManager), chatHandler.GetHistory)

		// WebSocket endpoint (handles authentication internally via token query param or authorization header)
		api.GET("/ws", wsHandler.HandleWS)
	}
}
