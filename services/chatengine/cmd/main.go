package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"social-network-system/services/chatengine/config"
)

func main() {
	// Load config
	cfg, err := config.Load(".")
	if err != nil {
		log.Fatalf("Failed to load configurations: %v", err)
	}

	// Initialize dependency injection
	worker, cleanup, err := InitializeWorker(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize worker: %v", err)
	}
	defer cleanup()

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker loop in goroutine
	go worker.Start(ctx)

	// Graceful shutdown handling
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Chat Engine Worker...")
	cancel() // Cancel context to stop consumer loops

	// Give it a moment to commit final messages
	time.Sleep(2 * time.Second)
	log.Println("Chat Engine Worker exited")
}
