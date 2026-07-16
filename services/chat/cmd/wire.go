//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"

	"social-network-system/pkg/database"
	"social-network-system/pkg/jwtutil"
	"social-network-system/pkg/kafka"
	"social-network-system/services/chat/config"
	deliveryhttp "social-network-system/services/chat/internal/delivery/http"
	"social-network-system/services/chat/internal/delivery/ws"
	mongorepo "social-network-system/services/chat/internal/repository/mongodb"
	redisrepo "social-network-system/services/chat/internal/repository/redis"
	"social-network-system/services/chat/internal/usecase"
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

func provideEngine() *gin.Engine {
	return gin.Default()
}

func provideKafkaProducer(cfg *config.Config) (kafka.Producer, func()) {
	producer := kafka.NewProducer(cfg.KafkaBrokers, cfg.KafkaTopicChatIncoming)
	cleanup := func() {
		_ = producer.Close()
	}
	return producer, cleanup
}

// InitializeApp initializes all dependencies and constructs the App.
func InitializeApp(cfg *config.Config) (*App, func(), error) {
	wire.Build(
		provideMongoClient,
		provideMongoDatabase,
		provideRedisClient,
		provideTokenManager,
		provideEngine,
		provideKafkaProducer,

		mongorepo.NewChatRepository,
		redisrepo.NewPresenceRepository,
		usecase.NewChatUseCase,
		deliveryhttp.NewChatHandler,
		ws.NewWSHandler,

		wire.Struct(new(App), "*"),
	)
	return nil, nil, nil
}
