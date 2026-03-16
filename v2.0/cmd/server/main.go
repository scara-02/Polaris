package main

import (
	"context"
	"log"
	"log/slog"
	"time"

	"github.com/Akashpg-M/polaris/internal/adapter/handler"
	"github.com/Akashpg-M/polaris/internal/adapter/repository"
	"github.com/Akashpg-M/polaris/internal/application/spatial"
	"github.com/Akashpg-M/polaris/internal/application/stream"
	"github.com/Akashpg-M/polaris/internal/application/orchestrator"
	"github.com/Akashpg-M/polaris/pkg/logger"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Initialize Enterprise Logging
	logger.Init()
	slog.Info("Booting Polaris v2.1 IoT Orchestration Engine...")

	// 2. Initialize the In-Memory Spatial Engine (The Brain)
	engine := spatial.NewEngine()
	slog.Info("In-Memory QuadTree Engine initialized.")
	registry := handler.NewConnectionRegistry()
	// 3. Initialize Infrastructure (Redis)
	redisURL := "redis://localhost:6379/0"
	
	// Publisher: For the Gateway to dump telemetry into Redis
	redisAdapter, err := repository.NewRedisStreamAdapter(redisURL)
	if err != nil {
		slog.Error("CRITICAL: Failed to connect to Redis Adapter", "error", err)
		log.Fatalf("System halted: %v", err)
	}

	// Consumer: For the background workers to pull telemetry out of Redis
	redisConsumer, err := stream.NewRedisConsumer(redisURL, engine)
	if err != nil {
		slog.Error("CRITICAL: Failed to initialize Redis Consumer", "error", err)
		log.Fatalf("System halted: %v", err)
	}

	// 4. Background Consumer Worker
	// We use a context so we can gracefully shut it down later if needed.
	ctx := context.Background()
	go redisConsumer.Start(ctx, "worker-alpha")
	slog.Info("Consumer Group 'worker-alpha' is actively polling the telemetry stream.")

	// Inject the policy into the orchestrator
	demandStrategy := &orchestrator.StaticZoneStrategy{}
	rebalancer := orchestrator.NewRebalancer(engine, registry, demandStrategy)
	go rebalancer.StartAutonomousLoop(ctx)

	// 5. Initialize the HTTP Handlers
	ingestionHandler := handler.NewIngestionHandler(redisAdapter, registry)
	matchHandler := handler.NewMatchHandler(engine)

	// 6. Setup the Gin Router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Configure CORS for the frontend dashboard
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 7. Register Routes
	// This is the universal endpoint for all IoT devices (Cars, Drones, Sensors)
	router.GET("/ws/telemetry", ingestionHandler.HandleIoTConnection)
	api := router.Group("/api/v1")
	{
		api.GET("/nodes/match", matchHandler.GetNearestNodes)
	}
	// 8. Start the Server
	port := ":6080"
	slog.Info("Gateway active. Listening for IoT connections", "port", port)
	if err := router.Run(port); err != nil {
		slog.Error("Server crashed", "error", err)
	}
}