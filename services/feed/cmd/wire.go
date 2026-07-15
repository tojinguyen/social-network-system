//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	redis2 "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"

	"social-network-system/pkg/database"
	"social-network-system/pkg/jwtutil"
	"social-network-system/services/feed/config"
	delivery "social-network-system/services/feed/internal/delivery/http"
	mongorepo "social-network-system/services/feed/internal/repository/mongodb"
	redisrepo "social-network-system/services/feed/internal/repository/redis"
	"social-network-system/services/feed/internal/usecase"
)

func provideMongoClient(cfg *config.Config) (*mongo.Client, func(), error) {
	client, _, err := database.ConnectMongoDB(context.Background(), cfg.MongoURI, cfg.MongoDBName)
	cleanup := func() {
		if client != nil {
			_ = client.Disconnect(context.Background())
		}
	}
	return client, cleanup, err
}

func provideMongoDatabase(client *mongo.Client, cfg *config.Config) *mongo.Database {
	return client.Database(cfg.MongoDBName)
}

func provideRedisClient(cfg *config.Config) (*redis2.Client, func(), error) {
	client, err := database.ConnectRedis(context.Background(), cfg.RedisURI, cfg.RedisPassword)
	cleanup := func() {
		if client != nil {
			_ = client.Close()
		}
	}
	return client, cleanup, err
}

func provideTokenManager(cfg *config.Config) jwtutil.TokenManager {
	return jwtutil.NewJWTManager(cfg.JWTSecret)
}

func provideEngine() *gin.Engine {
	return gin.Default()
}

// InitializeApp initializes all dependencies and constructs the App.
func InitializeApp(cfg *config.Config) (*App, func(), error) {
	wire.Build(
		provideMongoClient,
		provideMongoDatabase,
		provideRedisClient,
		provideTokenManager,
		provideEngine,

		mongorepo.NewFeedRepository,
		redisrepo.NewFeedCacheRepository,
		usecase.NewFeedUseCase,
		delivery.NewFeedHandler,

		wire.Struct(new(App), "*"),
	)
	return nil, nil, nil
}
