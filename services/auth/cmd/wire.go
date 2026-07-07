//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"go.mongodb.org/mongo-driver/mongo"
	"github.com/redis/go-redis/v9"

	"social-network-system/pkg/database"
	"social-network-system/pkg/hash"
	"social-network-system/pkg/jwtutil"
	"social-network-system/services/auth/config"
	delivery "social-network-system/services/auth/internal/delivery/http"
	mongorepo "social-network-system/services/auth/internal/repository/mongodb"
	redisrepo "social-network-system/services/auth/internal/repository/redis"
	"social-network-system/services/auth/internal/usecase"
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

func provideRedisClient(cfg *config.Config) (*redis.Client, func(), error) {
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

func providePasswordHasher() hash.PasswordHasher {
	return hash.NewBcryptHasher(0)
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
		providePasswordHasher,
		provideEngine,

		mongorepo.NewUserRepository,
		redisrepo.NewTokenRepository,
		usecase.NewAuthUseCase,
		delivery.NewAuthHandler,

		wire.Struct(new(App), "*"),
	)
	return nil, nil, nil
}
