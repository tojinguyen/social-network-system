//go:build wireinject
// +build wireinject

package main

import (
	"context"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"

	"social-network-system/pkg/database"
	"social-network-system/pkg/kafka"
	"social-network-system/services/chatengine/config"
	"social-network-system/services/chatengine/internal/worker"
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

func provideRedisClient(cfg *config.Config) (*redis.Client, func(), error) {
	client, err := database.ConnectRedis(context.Background(), cfg.RedisURI, cfg.RedisPassword)
	cleanup := func() {
		if client != nil {
			_ = client.Close()
		}
	}
	return client, cleanup, err
}

func provideKafkaConsumer(cfg *config.Config) (kafka.Consumer, func()) {
	consumer := kafka.NewConsumer(cfg.KafkaBrokers, cfg.ConsumerGroupID, cfg.KafkaTopicChatIncoming)
	cleanup := func() {
		_ = consumer.Close()
	}
	return consumer, cleanup
}

// InitializeWorker initializes all dependencies for Chat Engine Worker.
func InitializeWorker(cfg *config.Config) (*worker.ChatEngineWorker, func(), error) {
	wire.Build(
		provideMongoClient,
		provideRedisClient,
		provideKafkaConsumer,
		worker.NewChatEngineWorker,
	)
	return nil, nil, nil
}
