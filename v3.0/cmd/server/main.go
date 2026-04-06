// package main

// import (
// 	"context"
// 	"log"
// 	"log/slog"
// 	"time"

// 	"github.com/Akashpg-M/polaris/internal/adapter/handler"
// 	"github.com/Akashpg-M/polaris/internal/adapter/repository"
// 	"github.com/Akashpg-M/polaris/internal/application/spatial"
// 	"github.com/Akashpg-M/polaris/internal/application/stream"
// 	"github.com/Akashpg-M/polaris/internal/application/orchestrator"
// 	"github.com/Akashpg-M/polaris/algo_/logger"

// 	"github.com/gin-contrib/cors"
// 	"github.com/gin-gonic/gin"
// )

// func main() {
// 	// 1. Initialize Enterprise Logging
// 	logger.Init()
// 	slog.Info("Booting Polaris v2.1 IoT Orchestration Engine...")

// 	// 2. Initialize the In-Memory Spatial Engine (The Brain)
// 	engine := spatial.NewEngine()
// 	slog.Info("In-Memory QuadTree Engine initialized.")
// 	registry := handler.NewConnectionRegistry()
// 	// 3. Initialize Infrastructure (Redis)
// 	redisURL := "redis://localhost:6379/0"
	
// 	// Publisher: For the Gateway to dump telemetry into Redis
// 	redisAdapter, err := repository.NewRedisStreamAdapter(redisURL)
// 	if err != nil {
// 		slog.Error("CRITICAL: Failed to connect to Redis Adapter", "error", err)
// 		log.Fatalf("System halted: %v", err)
// 	}

// 	// Consumer: For the background workers to pull telemetry out of Redis
// 	redisConsumer, err := stream.NewRedisConsumer(redisURL, engine)
// 	if err != nil {
// 		slog.Error("CRITICAL: Failed to initialize Redis Consumer", "error", err)
// 		log.Fatalf("System halted: %v", err)
// 	}

// 	// Initialize the Database Archiver
	

// 	// 4. Background Consumer Worker
// 	// use a context so we can gracefully shut it down later if needed.
// 	ctx := context.Background()

// 	postgresURL := "postgres://polaris_user:polaris_password@localhost:5432/polaris_core?sslmode=disable"
// 	archiver, err := stream.NewPostgresArchiver(redisURL, postgresURL)
// 	if err != nil {
// 		slog.Error("Warning: Failed to connect to PostgreSQL. Running without history archiver.", "error", err)
// 	}else {
//     go archiver.Start(ctx)
// 		slog.Info("PostgreSQL Archiver is active. Recording telemetry history.")
// 	}

// 	go redisConsumer.Start(ctx, "worker-alpha")
// 	slog.Info("Consumer Group 'worker-alpha' is actively polling the telemetry stream.")

// 	// Inject the policy into the orchestrator
// 	demandStrategy := &orchestrator.StaticZoneStrategy{}
// 	rebalancer := orchestrator.NewRebalancer(engine, registry, demandStrategy)
// 	go rebalancer.StartAutonomousLoop(ctx)

// 	// 5. Initialize the HTTP Handlers
// 	ingestionHandler := handler.NewIngestionHandler(redisAdapter, registry)
// 	matchHandler := handler.NewMatchHandler(engine)

// 	// 6. Setup the Gin Router
// 	gin.SetMode(gin.ReleaseMode)
// 	router := gin.New()
// 	router.Use(gin.Recovery())

// 	// Configure CORS for the frontend dashboard
// 	router.Use(cors.New(cors.Config{
// 		AllowOrigins:     []string{"*"},
// 		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "OPTIONS"},
// 		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
// 		AllowCredentials: true,
// 		MaxAge:           12 * time.Hour,
// 	}))

// 	// 7. Register Routes
// 	// universal endpoint for all IoT devices (Cars, Drones, Sensors)
// 	router.GET("/ws/telemetry", ingestionHandler.HandleIoTConnection)
// 	api := router.Group("/api/v1")
// 	{
// 		api.GET("/nodes/match", matchHandler.GetNearestNodes)
// 	}
// 	// 8. Start the Server
// 	port := ":6080"
// 	slog.Info("Gateway active. Listening for IoT connections", "port", port)
// 	if err := router.Run(port); err != nil {
// 		slog.Error("Server crashed", "error", err)
// 	}
// }



package main

import (
	"os"
	"context"
	"encoding/json"
	"log"
	"log/slog"

	"github.com/Akashpg-M/polaris/algo_/logger"
	"github.com/Akashpg-M/polaris/internal/adapter/handler"
	"github.com/Akashpg-M/polaris/internal/adapter/repository"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger.Init()
	slog.Info("Booting Polaris v3.0 API Gateway...")

	redisURL := os.Getenv("REDIS_URL")
	
	// 1. Publisher for Ingestion
	redisAdapter, err := repository.NewRedisStreamAdapter(redisURL)
	if err != nil {
		log.Fatalf("System halted: %v", err)
	}

	registry := handler.NewConnectionRegistry()

	// 2. Subscriber for Commands (The Bridge from the Engine)
	go startCommandSubscriber(redisURL, registry)

	// 3. Setup Router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	ingestionHandler := handler.NewIngestionHandler(redisAdapter, registry)
	router.GET("/ws/telemetry", ingestionHandler.HandleIoTConnection)
	router.GET("/api/demand/forecast", func(c *gin.Context) {
        payload, err := FetchRiderForecast() // Ensure this helper is defined below
        if err != nil {
            slog.Error("GNN sidecar connection failed", "error", err)
            c.JSON(503, gin.H{"error": "GNN sidecar unavailable"})
            return
        }
        c.JSON(200, payload)
    })
	port := ":6080"
	slog.Info("Gateway active. Listening for IoT WebSockets", "port", port)
	router.Run(port)
}

// startCommandSubscriber listens for routing directives from the Engine
func startCommandSubscriber(redisURL string, registry *handler.ConnectionRegistry) {
	opts, _ := redis.ParseURL(redisURL)
	client := redis.NewClient(opts)
	pubsub := client.Subscribe(context.Background(), "telemetry:commands")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		var payload struct {
			NodeID  string      `json:"node_id"`
			Command interface{} `json:"command"`
		}
		if err := json.Unmarshal([]byte(msg.Payload), &payload); err == nil {
			// Push the command to the actual physical device
			registry.SendCommand(payload.NodeID, payload.Command)
		}
	}
}