package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"social-network-system/pkg/response"
	"social-network-system/services/auth/internal/domain"
	"social-network-system/services/auth/internal/usecase"
)

// AuthHandler handles HTTP requests for authentication.
type AuthHandler struct {
	useCase usecase.AuthUseCase
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(useCase usecase.AuthUseCase) *AuthHandler {
	return &AuthHandler{useCase: useCase}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req domain.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	err := h.useCase.Register(c.Request.Context(), &req)
	if err != nil {
		if err == usecase.ErrUserAlreadyExists {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "User registered successfully", nil)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	res, err := h.useCase.Login(c.Request.Context(), &req)
	if err != nil {
		if err == usecase.ErrInvalidCredentials {
			response.Error(c, http.StatusUnauthorized, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Login successful", res)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req domain.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	res, err := h.useCase.Refresh(c.Request.Context(), &req)
	if err != nil {
		if err == usecase.ErrInvalidToken {
			response.Error(c, http.StatusUnauthorized, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	type refreshResponse struct {
		AccessToken string `json:"access_token"`
	}

	response.Success(c, http.StatusOK, "Token refreshed successfully", refreshResponse{
		AccessToken: res.AccessToken,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req domain.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	err := h.useCase.Logout(c.Request.Context(), &req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Logged out successfully", nil)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "User context not found")
		return
	}

	response.Success(c, http.StatusOK, "Get user profile successful", gin.H{
		"user_id": userID,
	})
}
