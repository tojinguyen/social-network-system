package jwtutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTManager_GenerateAndVerifyAccessToken(t *testing.T) {
	secretKey := "super_secret_key"
	manager := NewJWTManager(secretKey)

	userID := "60d5ec49f333333333333333"
	duration := 15 * time.Minute

	t.Run("Success - Generate and Verify Token", func(t *testing.T) {
		token, err := manager.GenerateAccessToken(userID, duration)
		require.NoError(t, err)
		require.NotEmpty(t, token)

		claims, err := manager.VerifyAccessToken(token)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.Subject)
	})

	t.Run("Error - Expired Token", func(t *testing.T) {
		expiredToken, err := manager.GenerateAccessToken(userID, -1*time.Minute)
		require.NoError(t, err)

		claims, err := manager.VerifyAccessToken(expiredToken)
		assert.ErrorIs(t, err, ErrExpiredToken)
		assert.Nil(t, claims)
	})

	t.Run("Error - Wrong Secret Key", func(t *testing.T) {
		token, err := manager.GenerateAccessToken(userID, duration)
		require.NoError(t, err)

		wrongManager := NewJWTManager("wrong_secret_key")
		claims, err := wrongManager.VerifyAccessToken(token)
		assert.ErrorIs(t, err, ErrInvalidToken)
		assert.Nil(t, claims)
	})

	t.Run("Error - Malformed Token", func(t *testing.T) {
		claims, err := manager.VerifyAccessToken("invalid.token.string")
		assert.ErrorIs(t, err, ErrInvalidToken)
		assert.Nil(t, claims)
	})
}

func TestJWTManager_GenerateRefreshToken(t *testing.T) {
	manager := NewJWTManager("secret")

	token1, err := manager.GenerateRefreshToken()
	require.NoError(t, err)
	require.NotEmpty(t, token1)

	token2, err := manager.GenerateRefreshToken()
	require.NoError(t, err)
	require.NotEmpty(t, token2)

	assert.NotEqual(t, token1, token2, "Refresh tokens should be unique random UUIDs")
}
