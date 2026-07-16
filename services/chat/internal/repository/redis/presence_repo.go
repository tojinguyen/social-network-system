package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"social-network-system/services/chat/internal/domain"
)

type presenceRepo struct {
	client *redis.Client
}

// NewPresenceRepository creates a new PresenceRepository instance using Redis.
func NewPresenceRepository(client *redis.Client) domain.PresenceRepository {
	return &presenceRepo{client: client}
}

func (r *presenceRepo) SetOnline(ctx context.Context, userID string, nodeID string, ttl time.Duration) error {
	key := fmt.Sprintf("presence:user:%s", userID)
	return r.client.Set(ctx, key, nodeID, ttl).Err()
}

func (r *presenceRepo) SetOffline(ctx context.Context, userID string, nodeID string) error {
	key := fmt.Sprintf("presence:user:%s", userID)

	// Use Lua script to safely delete only if the current node holds the session
	// to avoid race condition where client has reconnected to another node.
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	return r.client.Eval(ctx, script, []string{key}, nodeID).Err()
}

func (r *presenceRepo) GetNode(ctx context.Context, userID string) (string, error) {
	key := fmt.Sprintf("presence:user:%s", userID)
	nodeID, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Not online
	}
	return nodeID, err
}
