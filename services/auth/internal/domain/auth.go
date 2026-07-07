package domain

import (
	"context"
	"time"
)

// RegisterRequest defines the input fields required for user registration.
type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=30"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginRequest defines the input fields required for user login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// TokenResponse represents the authentication payload returned upon successful login.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// RefreshRequest defines the input fields required to refresh the access token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutRequest defines the input fields required to log out.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// TokenRepository defines the contract for operations on session tokens stored in Redis.
type TokenRepository interface {
	StoreRefreshToken(ctx context.Context, token string, userID string, ttl time.Duration) error
	GetUserIDByRefreshToken(ctx context.Context, token string) (string, error)
	DeleteRefreshToken(ctx context.Context, token string) error
}
