//go:build integration

package redis

import (
	"context"
	"testing"
	"time"

	redisClient "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcRedis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func setupTestRedis(t *testing.T) (*redisClient.Client, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	redisContainer, err := tcRedis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err)

	endpoint, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := redisClient.ParseURL(endpoint)
	require.NoError(t, err)

	rdb := redisClient.NewClient(opts)

	cleanup := func() {
		_ = rdb.Close()
		_ = redisContainer.Terminate(context.Background())
	}

	return rdb, cleanup
}

func TestTokenRepository_Integration(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	repo := NewTokenRepository(rdb)
	ctx := context.Background()

	token := "sample-refresh-token-uuid"
	userID := "60d5ec49f333333333333333"

	t.Run("Store and Get Refresh Token", func(t *testing.T) {
		err := repo.StoreRefreshToken(ctx, token, userID, 1*time.Hour)
		require.NoError(t, err)

		fetchedID, err := repo.GetUserIDByRefreshToken(ctx, token)
		require.NoError(t, err)
		assert.Equal(t, userID, fetchedID)
	})

	t.Run("Get Non-existent Token", func(t *testing.T) {
		fetchedID, err := repo.GetUserIDByRefreshToken(ctx, "non-existent")
		require.NoError(t, err)
		assert.Empty(t, fetchedID)
	})

	t.Run("Delete Refresh Token", func(t *testing.T) {
		err := repo.DeleteRefreshToken(ctx, token)
		require.NoError(t, err)

		fetchedID, err := repo.GetUserIDByRefreshToken(ctx, token)
		require.NoError(t, err)
		assert.Empty(t, fetchedID)
	})
}
