package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"social-network-system/pkg/jwtutil"
	"social-network-system/services/auth/internal/domain"
	"social-network-system/services/auth/internal/usecase"
)

type mockAuthUseCase struct {
	registerFunc func(ctx context.Context, req *domain.RegisterRequest) error
	loginFunc    func(ctx context.Context, req *domain.LoginRequest) (*domain.TokenResponse, error)
	refreshFunc  func(ctx context.Context, req *domain.RefreshRequest) (*domain.TokenResponse, error)
	logoutFunc   func(ctx context.Context, req *domain.LogoutRequest) error
}

func (m *mockAuthUseCase) Register(ctx context.Context, req *domain.RegisterRequest) error {
	if m.registerFunc != nil {
		return m.registerFunc(ctx, req)
	}
	return nil
}

func (m *mockAuthUseCase) Login(ctx context.Context, req *domain.LoginRequest) (*domain.TokenResponse, error) {
	if m.loginFunc != nil {
		return m.loginFunc(ctx, req)
	}
	return &domain.TokenResponse{AccessToken: "access_token", RefreshToken: "refresh_token"}, nil
}

func (m *mockAuthUseCase) Refresh(ctx context.Context, req *domain.RefreshRequest) (*domain.TokenResponse, error) {
	if m.refreshFunc != nil {
		return m.refreshFunc(ctx, req)
	}
	return &domain.TokenResponse{AccessToken: "new_access_token", RefreshToken: req.RefreshToken}, nil
}

func (m *mockAuthUseCase) Logout(ctx context.Context, req *domain.LogoutRequest) error {
	if m.logoutFunc != nil {
		return m.logoutFunc(ctx, req)
	}
	return nil
}

func setupTestEngine(uc usecase.AuthUseCase, tokenManager jwtutil.TokenManager) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := NewAuthHandler(uc)
	SetupRouter(r, handler, tokenManager)
	return r
}

func TestAuthHandler_Register(t *testing.T) {
	tokenManager := jwtutil.NewJWTManager("test_secret")

	t.Run("Status 201 - Created", func(t *testing.T) {
		uc := &mockAuthUseCase{}
		router := setupTestEngine(uc, tokenManager)

		body := domain.RegisterRequest{
			Username: "john_doe",
			Email:    "john@example.com",
			Password: "securepassword",
		}
		jsonBytes, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, true, resp["success"])
		assert.Equal(t, "User registered successfully", resp["message"])
	})

	t.Run("Status 400 - Invalid Request Validation", func(t *testing.T) {
		uc := &mockAuthUseCase{}
		router := setupTestEngine(uc, tokenManager)

		body := domain.RegisterRequest{
			Username: "jo",                // Min 3 chars
			Email:    "invalid-email",     // Invalid email format
			Password: "123",               // Min 6 chars
		}
		jsonBytes, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("Status 409 - User Already Exists", func(t *testing.T) {
		uc := &mockAuthUseCase{
			registerFunc: func(ctx context.Context, req *domain.RegisterRequest) error {
				return usecase.ErrUserAlreadyExists
			},
		}
		router := setupTestEngine(uc, tokenManager)

		body := domain.RegisterRequest{
			Username: "john_doe",
			Email:    "john@example.com",
			Password: "securepassword",
		}
		jsonBytes, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})
}

func TestAuthHandler_Login(t *testing.T) {
	tokenManager := jwtutil.NewJWTManager("test_secret")

	t.Run("Status 200 - OK Login Success", func(t *testing.T) {
		uc := &mockAuthUseCase{
			loginFunc: func(ctx context.Context, req *domain.LoginRequest) (*domain.TokenResponse, error) {
				return &domain.TokenResponse{
					AccessToken:  "mocked_access_token",
					RefreshToken: "mocked_refresh_token",
				}, nil
			},
		}
		router := setupTestEngine(uc, tokenManager)

		body := domain.LoginRequest{
			Email:    "john@example.com",
			Password: "securepassword",
		}
		jsonBytes, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, true, resp["success"])
		data := resp["data"].(map[string]interface{})
		assert.Equal(t, "mocked_access_token", data["access_token"])
		assert.Equal(t, "mocked_refresh_token", data["refresh_token"])
	})

	t.Run("Status 401 - Invalid Credentials", func(t *testing.T) {
		uc := &mockAuthUseCase{
			loginFunc: func(ctx context.Context, req *domain.LoginRequest) (*domain.TokenResponse, error) {
				return nil, usecase.ErrInvalidCredentials
			},
		}
		router := setupTestEngine(uc, tokenManager)

		body := domain.LoginRequest{
			Email:    "john@example.com",
			Password: "wrongpassword",
		}
		jsonBytes, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestAuthHandler_Refresh(t *testing.T) {
	tokenManager := jwtutil.NewJWTManager("test_secret")

	t.Run("Status 200 - OK Token Refresh", func(t *testing.T) {
		uc := &mockAuthUseCase{
			refreshFunc: func(ctx context.Context, req *domain.RefreshRequest) (*domain.TokenResponse, error) {
				return &domain.TokenResponse{
					AccessToken:  "new_refreshed_access_token",
					RefreshToken: req.RefreshToken,
				}, nil
			},
		}
		router := setupTestEngine(uc, tokenManager)

		body := domain.RefreshRequest{RefreshToken: "valid_refresh_token"}
		jsonBytes, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data := resp["data"].(map[string]interface{})
		assert.Equal(t, "new_refreshed_access_token", data["access_token"])
	})

	t.Run("Status 401 - Invalid Refresh Token", func(t *testing.T) {
		uc := &mockAuthUseCase{
			refreshFunc: func(ctx context.Context, req *domain.RefreshRequest) (*domain.TokenResponse, error) {
				return nil, usecase.ErrInvalidToken
			},
		}
		router := setupTestEngine(uc, tokenManager)

		body := domain.RefreshRequest{RefreshToken: "expired_token"}
		jsonBytes, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestAuthHandler_Logout(t *testing.T) {
	tokenManager := jwtutil.NewJWTManager("test_secret")

	t.Run("Status 200 - OK Logout", func(t *testing.T) {
		uc := &mockAuthUseCase{}
		router := setupTestEngine(uc, tokenManager)

		body := domain.LogoutRequest{RefreshToken: "active_refresh_token"}
		jsonBytes, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Status 500 - Internal Server Error", func(t *testing.T) {
		uc := &mockAuthUseCase{
			logoutFunc: func(ctx context.Context, req *domain.LogoutRequest) error {
				return errors.New("db error")
			},
		}
		router := setupTestEngine(uc, tokenManager)

		body := domain.LogoutRequest{RefreshToken: "active_refresh_token"}
		jsonBytes, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestAuthHandler_Me(t *testing.T) {
	tokenManager := jwtutil.NewJWTManager("test_secret")
	userID := "60d5ec49f333333333333333"
	accessToken, err := tokenManager.GenerateAccessToken(userID, 15*time.Minute)
	require.NoError(t, err)

	uc := &mockAuthUseCase{}
	router := setupTestEngine(uc, tokenManager)

	t.Run("Status 200 - Get Profile Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		data := resp["data"].(map[string]interface{})
		assert.Equal(t, userID, data["user_id"])
	})

	t.Run("Status 401 - Missing Authorization Header", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
