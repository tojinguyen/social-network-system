//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	redisClient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcMongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	tcRedis "github.com/testcontainers/testcontainers-go/modules/redis"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"social-network-system/pkg/hash"
	"social-network-system/pkg/jwtutil"
	authHttp "social-network-system/services/auth/internal/delivery/http"
	"social-network-system/services/auth/internal/domain"
	mongoRepo "social-network-system/services/auth/internal/repository/mongodb"
	redisRepo "social-network-system/services/auth/internal/repository/redis"
	"social-network-system/services/auth/internal/usecase"
)

func setupIntegrationEnvironment(t *testing.T) (*gin.Engine, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// 1. Start MongoDB Container
	mongoContainer, err := tcMongo.Run(ctx, "mongo:6.0")
	require.NoError(t, err)

	mongoEndpoint, err := mongoContainer.ConnectionString(ctx)
	require.NoError(t, err)

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoEndpoint))
	require.NoError(t, err)

	mongoDB := mongoClient.Database("integration_auth_db")

	// 2. Start Redis Container
	redisContainer, err := tcRedis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err)

	redisEndpoint, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err)

	redisOpts, err := redisClient.ParseURL(redisEndpoint)
	require.NoError(t, err)

	rdb := redisClient.NewClient(redisOpts)

	// 3. Construct Clean Architecture Layers
	userRepo := mongoRepo.NewUserRepository(mongoDB)
	tokenRepo := redisRepo.NewTokenRepository(rdb)
	hasher := hash.NewBcryptHasher(0)
	jwtManager := jwtutil.NewJWTManager("integration_test_secret")

	authUC := usecase.NewAuthUseCase(userRepo, tokenRepo, hasher, jwtManager)
	handler := authHttp.NewAuthHandler(authUC)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	authHttp.SetupRouter(router, handler, jwtManager)

	cleanup := func() {
		_ = mongoClient.Disconnect(context.Background())
		_ = rdb.Close()
		_ = mongoContainer.Terminate(context.Background())
		_ = redisContainer.Terminate(context.Background())
	}

	return router, cleanup
}

func TestAuthAPI_E2E_IntegrationFlow(t *testing.T) {
	router, cleanup := setupIntegrationEnvironment(t)
	defer cleanup()

	var accessToken string
	var refreshToken string

	// Step 1: Register User
	t.Run("Step 1: Register New User", func(t *testing.T) {
		regBody := domain.RegisterRequest{
			Username: "e2e_user",
			Email:    "e2e@example.com",
			Password: "securePassword123",
		}
		jsonBytes, _ := json.Marshal(regBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	// Step 2: Login User
	t.Run("Step 2: Login User", func(t *testing.T) {
		loginBody := domain.LoginRequest{
			Email:    "e2e@example.com",
			Password: "securePassword123",
		}
		jsonBytes, _ := json.Marshal(loginBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		data := resp["data"].(map[string]interface{})
		accessToken = data["access_token"].(string)
		refreshToken = data["refresh_token"].(string)

		require.NotEmpty(t, accessToken)
		require.NotEmpty(t, refreshToken)
	})

	// Step 3: Access Protected Route /me
	t.Run("Step 3: Access /me Protected Endpoint", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		data := resp["data"].(map[string]interface{})
		assert.NotEmpty(t, data["user_id"])
	})

	// Step 4: Refresh Access Token
	t.Run("Step 4: Refresh Access Token", func(t *testing.T) {
		refreshBody := domain.RefreshRequest{
			RefreshToken: refreshToken,
		}
		jsonBytes, _ := json.Marshal(refreshBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		data := resp["data"].(map[string]interface{})
		newAccessToken := data["access_token"].(string)
		require.NotEmpty(t, newAccessToken)
	})

	// Step 5: Logout User
	t.Run("Step 5: Logout User", func(t *testing.T) {
		logoutBody := domain.LogoutRequest{
			RefreshToken: refreshToken,
		}
		jsonBytes, _ := json.Marshal(logoutBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Step 6: Refresh Attempt After Logout (Expect Failure)
	t.Run("Step 6: Refresh Token After Logout Fails", func(t *testing.T) {
		refreshBody := domain.RefreshRequest{
			RefreshToken: refreshToken,
		}
		jsonBytes, _ := json.Marshal(refreshBody)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
