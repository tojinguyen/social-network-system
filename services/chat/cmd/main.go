package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"social-network-system/pkg/jwtutil"
	"social-network-system/pkg/logger"
	"social-network-system/pkg/metrics"
	"social-network-system/pkg/middleware"
	"social-network-system/pkg/tracing"
	"social-network-system/services/chat/config"
	deliveryhttp "social-network-system/services/chat/internal/delivery/http"
	"social-network-system/services/chat/internal/delivery/ws"
)

// App aggregates all initialized structures for the service.
type App struct {
	Config       *config.Config
	Engine       *gin.Engine
	MongoClient  *mongo.Client
	RedisClient  *redis.Client
	ChatHandler  *deliveryhttp.ChatHandler
	WSHandler    *ws.WSHandler
	TokenManager jwtutil.TokenManager
}

func main() {
	// Initialize logger
	logger.Init("chat-service")

	// Load config
	cfg, err := config.Load(".")
	if err != nil {
		log.Fatalf("Failed to load configurations: %v", err)
	}

	// Initialize tracing
	if os.Getenv("OTEL_ENABLED") == "true" {
		tpShutdown, err := tracing.InitTracer(context.Background(), "chat-service")
		if err != nil {
			log.Fatalf("Failed to initialize tracer: %v", err)
		}
		defer tpShutdown()
	}

	// Initialize metrics
	var metricsExporter http.Handler
	if os.Getenv("OTEL_METRICS_ENABLED") == "true" {
		exporter, shutdownMetrics, err := metrics.InitMetrics(context.Background(), "chat-service")
		if err != nil {
			log.Fatalf("Failed to initialize metrics: %v", err)
		}
		defer shutdownMetrics()
		metricsExporter = exporter
	}

	// Initialize dependencies using Google Wire
	app, cleanup, err := InitializeApp(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}
	defer cleanup()

	// Configure routing
	if os.Getenv("OTEL_ENABLED") == "true" {
		app.Engine.Use(otelgin.Middleware("chat-service"))
	}
	app.Engine.Use(middleware.LoggerMiddleware())

	if os.Getenv("OTEL_METRICS_ENABLED") == "true" && metricsExporter != nil {
		app.Engine.GET("/metrics", gin.WrapH(metricsExporter))
	}
	deliveryhttp.SetupRouter(app.Engine, app.ChatHandler, app.WSHandler, app.TokenManager)

	port := app.Config.Port
	if port == "" {
		port = "8084"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: app.Engine,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		log.Printf("Chat Service is running on port %s (Node ID: %s)", port, app.WSHandler.NodeID())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to run HTTP/WS server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
