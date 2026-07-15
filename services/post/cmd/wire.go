//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
	"go.mongodb.org/mongo-driver/mongo"

	"social-network-system/pkg/database"
	"social-network-system/pkg/jwtutil"
	"social-network-system/pkg/kafka"
	"social-network-system/services/post/config"
	delivery "social-network-system/services/post/internal/delivery/http"
	mongorepo "social-network-system/services/post/internal/repository/mongodb"
	"social-network-system/services/post/internal/usecase"
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

func provideTokenManager(cfg *config.Config) jwtutil.TokenManager {
	return jwtutil.NewJWTManager(cfg.JWTSecret)
}

func provideEngine() *gin.Engine {
	return gin.Default()
}

func provideKafkaProducer(cfg *config.Config) (kafka.Producer, func()) {
	producer := kafka.NewProducer(cfg.KafkaBrokers, cfg.PostCreatedTopic)
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
		provideTokenManager,
		provideEngine,
		provideKafkaProducer,

		mongorepo.NewPostRepository,
		mongorepo.NewFollowRepository,
		usecase.NewPostUseCase,
		usecase.NewFollowUseCase,
		delivery.NewPostHandler,
		delivery.NewFollowHandler,

		wire.Struct(new(App), "*"),
	)
	return nil, nil, nil
}
