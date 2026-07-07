package main

import (
	"log"
	"net/http"

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

	log.Printf("Auth Service is running on port %s", port)
	if err := http.ListenAndServe(":"+port, app.Engine); err != nil {
		log.Fatalf("Failed to run HTTP server: %v", err)
	}
}
