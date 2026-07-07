package database

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// ConnectRedis establishes a connection to Redis.
func ConnectRedis(ctx context.Context, addr string, password string) (*redis.Client, error) {
	client := redis.NewClient(optionsRedis(addr, password))

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}

func optionsRedis(addr, password string) *redis.Options {
	return &redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0, // Default DB
	}
}
