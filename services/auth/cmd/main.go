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
	"go.mongodb.org/mongo-driver/mongo"
	"github.com/redis/go-redis/v9"

	"social-network-system/pkg/jwtutil"
	"social-network-system/services/auth/config"
	delivery "social-network-system/services/auth/internal/delivery/http"
)

// App aggregates all initialized structures for the service.
type App struct {
	Config       *config.Config
	Engine       *gin.Engine
	MongoClient  *mongo.Client
	RedisClient  *redis.Client
	AuthHandler  *delivery.AuthHandler
	TokenManager jwtutil.TokenManager
}

func main() {
	// Load config using Viper from root or current directory
	cfg, err := config.Load(".")
	if err != nil {
		log.Fatalf("Failed to load configurations: %v", err)
	}

	// Initialize dependencies using Google Wire
	app, cleanup, err := InitializeApp(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}
	defer cleanup()

	// Configure routing
	delivery.SetupRouter(app.Engine, app.AuthHandler, app.TokenManager)

	port := app.Config.Port
	if port == "" {
		port = "8081"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: app.Engine,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		log.Printf("Auth Service is running on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to run HTTP server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be caught, so no need to add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
