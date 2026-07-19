package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"social-network-system/pkg/logger"
	"social-network-system/pkg/metrics"
	"social-network-system/pkg/tracing"
	"social-network-system/services/chatengine/config"
)

func main() {
	// Initialize logger
	logger.Init("chatengine-worker")

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

	// Initialize tracing
	if os.Getenv("OTEL_ENABLED") == "true" {
		tpShutdown, err := tracing.InitTracer(ctx, "chatengine-worker")
		if err != nil {
			log.Fatalf("Failed to initialize tracer: %v", err)
		}
		defer tpShutdown()
	}

	// Initialize metrics
	if os.Getenv("OTEL_METRICS_ENABLED") == "true" {
		metricsExporter, shutdownMetrics, err := metrics.InitMetrics(ctx, "chatengine-worker")
		if err == nil && metricsExporter != nil {
			defer shutdownMetrics()
			go func() {
				metricsPort := os.Getenv("METRICS_PORT")
				if metricsPort == "" {
					metricsPort = "8086"
				}
				log.Printf("Serving metrics on :%s/metrics", metricsPort)
				mux := http.NewServeMux()
				mux.Handle("/metrics", metricsExporter)
				if err := http.ListenAndServe(":" + metricsPort, mux); err != nil {
					log.Printf("Metrics HTTP server failed: %v", err)
				}
			}()
		}
	}

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
