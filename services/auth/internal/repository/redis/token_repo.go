package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"social-network-system/services/auth/internal/domain"
)

type tokenRepo struct {
	client *redis.Client
}

// NewTokenRepository creates a new instance of TokenRepository using Redis.
func NewTokenRepository(client *redis.Client) domain.TokenRepository {
	return &tokenRepo{client: client}
}

func (r *tokenRepo) StoreRefreshToken(ctx context.Context, token string, userID string, ttl time.Duration) error {
	key := fmt.Sprintf("refresh_token:%s", token)
	return r.client.Set(ctx, key, userID, ttl).Err()
}

func (r *tokenRepo) GetUserIDByRefreshToken(ctx context.Context, token string) (string, error) {
	key := fmt.Sprintf("refresh_token:%s", token)
	userID, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}
	return userID, nil
}

func (r *tokenRepo) DeleteRefreshToken(ctx context.Context, token string) error {
	key := fmt.Sprintf("refresh_token:%s", token)
	return r.client.Del(ctx, key).Err()
}
