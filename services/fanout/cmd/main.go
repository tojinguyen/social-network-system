package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"social-network-system/pkg/database"
	"social-network-system/pkg/kafka"
	"social-network-system/services/fanout/config"
	"social-network-system/services/fanout/internal/worker"
)

func main() {
	// Load config
	cfg, err := config.Load(".")
	if err != nil {
		log.Fatalf("Failed to load configurations: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect MongoDB
	mongoClient, _, err := database.ConnectMongoDB(ctx, cfg.MongoURI, cfg.MongoDBName)
	if err != nil {
		log.Fatalf("Failed to connect MongoDB: %v", err)
	}
	defer func() {
		_ = mongoClient.Disconnect(context.Background())
	}()

	// Connect Redis
	redisClient, err := database.ConnectRedis(ctx, cfg.RedisURI, cfg.RedisPassword)
	if err != nil {
		log.Fatalf("Failed to connect Redis: %v", err)
	}
	defer func() {
		_ = redisClient.Close()
	}()

	// Create Kafka Consumer
	consumer := kafka.NewConsumer(cfg.KafkaBrokers, "fanout-worker-group", cfg.PostCreatedTopic)
	defer func() {
		_ = consumer.Close()
	}()

	// Start worker
	fanoutWorker := worker.NewFanoutWorker(cfg, mongoClient, redisClient, consumer)
	go fanoutWorker.Start(ctx)

	log.Println("Fan-out Worker is running...")

	// Listen for shutdown signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Fan-out Worker...")
	cancel()

	// Wait 2 seconds for worker execution block exit
	time.Sleep(2 * time.Second)
	log.Println("Fan-out Worker stopped.")
}
