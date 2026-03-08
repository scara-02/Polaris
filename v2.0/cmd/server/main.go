package main

import (
	"log"
	"log/slog"
	"time"

	"github.com/Akashpg-M/polaris/internal/adapter/handler"
	"github.com/Akashpg-M/polaris/internal/adapter/repository"
	"github.com/Akashpg-M/polaris/pkg/logger"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Initialize Enterprise Logging
	logger.Init()
	slog.Info("Booting Polaris v2.0 Ingestion Gateway...")

	// 2. Initialize Infrastructure (Redis)
	// Make sure your Docker container is running!
	redisURL := "redis://localhost:6379/0"
	redisAdapter, err := repository.NewRedisStreamAdapter(redisURL)
	if err != nil {
		slog.Error("CRITICAL: Failed to connect to Redis", "error", err)
		log.Fatalf("System halted: %v", err)
	}
	slog.Info("Connected to Redis Event Stream buffer.")

	// 3. Initialize the HTTP Handlers
	// We inject the Redis Adapter into the WebSocket handler
	ingestionHandler := handler.NewIngestionHandler(redisAdapter)

	// 4. Setup the Gin Router
	gin.SetMode(gin.ReleaseMode) // Turn off Gin's noisy debug logs
	router := gin.New()
	router.Use(gin.Recovery())   // Prevent panics from crashing the server

	// Configure CORS for the frontend dashboard
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 5. Register Routes
	// This is the universal endpoint for all IoT devices (Cars, Drones, Sensors)
	router.GET("/ws/telemetry", ingestionHandler.HandleIoTConnection)

	// 6. Start the Server
	port := ":6080"
	slog.Info("Gateway active. Listening for IoT connections", "port", port)
	if err := router.Run(port); err != nil {
		slog.Error("Server crashed", "error", err)
	}
}