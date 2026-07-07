package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"social-network-system/pkg/jwtutil"
	"social-network-system/pkg/response"
)

// AuthMiddleware creates a Gin middleware that validates JWT access tokens.
func AuthMiddleware(tokenManager jwtutil.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, "Authorization header is required")
			c.Abort()
			return
		}

		fields := strings.Fields(authHeader)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "Bearer") {
			response.Error(c, http.StatusUnauthorized, "Invalid authorization header format")
			c.Abort()
			return
		}

		accessToken := fields[1]
		claims, err := tokenManager.VerifyAccessToken(accessToken)
		if err != nil {
			if err == jwtutil.ErrExpiredToken {
				response.Error(c, http.StatusUnauthorized, "Access token has expired")
			} else {
				response.Error(c, http.StatusUnauthorized, "Invalid access token")
			}
			c.Abort()
			return
		}

		// Inject user ID (Subject) into context
		c.Set("user_id", claims.Subject)
		c.Next()
	}
}
