package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"social-network-system/services/feed/internal/domain"
)

type feedCacheRepo struct {
	client *redis.Client
}

// NewFeedCacheRepository creates a new FeedCacheRepository instance using Redis.
func NewFeedCacheRepository(client *redis.Client) domain.FeedCacheRepository {
	return &feedCacheRepo{client: client}
}

func (r *feedCacheRepo) GetFeedCache(ctx context.Context, userID string, cursor time.Time, limit int) ([]string, error) {
	key := fmt.Sprintf("feed:user:%s", userID)

	max := "+inf"
	if !cursor.IsZero() {
		// Only retrieve posts with score strictly lower than the cursor timestamp score
		// UnixNano() represents the score used in Fan-out Worker
		max = strconv.FormatInt(cursor.UnixNano()-1, 10)
	}

	opt := redis.ZRangeBy{
		Min:    "-inf",
		Max:    max,
		Offset: 0,
		Count:  int64(limit),
	}

	// Fetch members sorted from high to low score (newest to oldest)
	return r.client.ZRevRangeByScore(ctx, key, &opt).Result()
}
