package http

import (
	"github.com/gin-gonic/gin"
	"social-network-system/pkg/jwtutil"
	"social-network-system/pkg/middleware"
)

// SetupRouter configures routes for the Post Service.
func SetupRouter(r *gin.Engine, postHandler *PostHandler, followHandler *FollowHandler, tokenManager jwtutil.TokenManager) {
	// All routes require authentication
	api := r.Group("/api/v1")
	api.Use(middleware.AuthMiddleware(tokenManager))
	{
		// Post routes
		posts := api.Group("/posts")
		{
			posts.POST("", postHandler.CreatePost)
			posts.GET("/:id", postHandler.GetPost)
			posts.DELETE("/:id", postHandler.DeletePost)
		}

		// Follow routes
		follows := api.Group("/follows")
		{
			follows.POST("", followHandler.Follow)
			follows.DELETE("/:target_id", followHandler.Unfollow)
			follows.GET("/following", followHandler.GetFollowing)
			follows.GET("/followers", followHandler.GetFollowers)
		}
	}
}
