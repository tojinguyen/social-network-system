package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"social-network-system/pkg/jwtutil"
	"social-network-system/pkg/response"
)

func setupTestRouter(tokenManager jwtutil.TokenManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", AuthMiddleware(tokenManager), func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		response.Success(c, http.StatusOK, "OK", gin.H{"user_id": userID})
	})
	return r
}

func TestAuthMiddleware(t *testing.T) {
	secretKey := "middleware_test_secret"
	tokenManager := jwtutil.NewJWTManager(secretKey)
	router := setupTestRouter(tokenManager)

	validUserID := "60d5ec49f333333333333333"
	validToken, err := tokenManager.GenerateAccessToken(validUserID, 15*time.Minute)
	require.NoError(t, err)

	expiredToken, err := tokenManager.GenerateAccessToken(validUserID, -1*time.Minute)
	require.NoError(t, err)

	tests := []struct {
		name           string
		authHeader     string
		setHeader      bool
		expectedStatus int
		expectedMsg    string
		checkUserID    bool
	}{
		{
			name:           "Success - Valid Bearer Token",
			authHeader:     "Bearer " + validToken,
			setHeader:      true,
			expectedStatus: http.StatusOK,
			checkUserID:    true,
		},
		{
			name:           "Error - Missing Authorization Header",
			authHeader:     "",
			setHeader:      false,
			expectedStatus: http.StatusUnauthorized,
			expectedMsg:    "Authorization header is required",
		},
		{
			name:           "Error - Invalid Format No Bearer Prefix",
			authHeader:     validToken,
			setHeader:      true,
			expectedStatus: http.StatusUnauthorized,
			expectedMsg:    "Invalid authorization header format",
		},
		{
			name:           "Error - Invalid Format Scheme Basic",
			authHeader:     "Basic " + validToken,
			setHeader:      true,
			expectedStatus: http.StatusUnauthorized,
			expectedMsg:    "Invalid authorization header format",
		},
		{
			name:           "Error - Expired Access Token",
			authHeader:     "Bearer " + expiredToken,
			setHeader:      true,
			expectedStatus: http.StatusUnauthorized,
			expectedMsg:    "Access token has expired",
		},
		{
			name:           "Error - Invalid Token String",
			authHeader:     "Bearer invalid.token.string",
			setHeader:      true,
			expectedStatus: http.StatusUnauthorized,
			expectedMsg:    "Invalid access token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
			if tt.setHeader {
				req.Header.Set("Authorization", tt.authHeader)
			}

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var res map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &res)
			require.NoError(t, err)

			if tt.expectedMsg != "" {
				assert.Equal(t, tt.expectedMsg, res["message"])
			}
			if tt.checkUserID {
				data := res["data"].(map[string]interface{})
				assert.Equal(t, validUserID, data["user_id"])
			}
		})
	}
}
