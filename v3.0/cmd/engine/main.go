package main

import (
	"os"
	"context"
	"encoding/json"
	"log/slog"

	"github.com/Akashpg-M/polaris/algo_/logger"
	"github.com/Akashpg-M/polaris/internal/adapter/handler"
	"github.com/Akashpg-M/polaris/internal/application/orchestrator"
	"github.com/Akashpg-M/polaris/internal/application/spatial"
	"github.com/Akashpg-M/polaris/internal/application/stream"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"os/signal"
	"syscall"
	"net/http"
	"time"
)

// RedisCommander implements ports.FleetCommander by publishing to Redis Pub/Sub
type RedisCommander struct {
	client *redis.Client
}

func (r *RedisCommander) SendCommand(nodeID string, payload interface{}) error {
	msg := map[string]interface{}{
		"node_id": nodeID,
		"command": payload,
	}
	data, _ := json.Marshal(msg)
	return r.client.Publish(context.Background(), "telemetry:commands", string(data)).Err()
}

func main() {
	logger.Init()
	slog.Info("Booting Polaris v3.0 Spatial Engine...")

	engine := spatial.NewEngine()
	redisURL := os.Getenv("REDIS_URL")
	postgresURL := os.Getenv("POSTGRES_URL")
	// ctx := context.Background()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()


	// 1. Consumers & Archivers
	redisConsumer, _ := stream.NewRedisConsumer(redisURL, engine)
	go redisConsumer.Start(ctx, "engine-node-1")

	archiver, _ := stream.NewPostgresArchiver(redisURL, postgresURL)
	go archiver.Start(ctx)

	// 2. Orchestrator with RedisCommander
	opts, _ := redis.ParseURL(redisURL)
	redisClient := redis.NewClient(opts)
	commander := &RedisCommander{client: redisClient}

	predictiveStrategy, err := orchestrator.NewPredictiveZoneStrategy(postgresURL)
	if err != nil {
		slog.Error("Failed to init predictive strategy", "error", err)
		panic(err)
	}

	rebalancer := orchestrator.NewRebalancer(engine, commander, predictiveStrategy)
	go rebalancer.StartAutonomousLoop(ctx)

	// 3. Setup Router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(cors.Default())

	matchHandler := handler.NewMatchHandler(engine)

	api := router.Group("/api/v1")
	{
		api.GET("/nodes/match", matchHandler.GetNearestNodes)

		api.GET("/zones/predicted", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"status": "success",
				"data": []map[string]interface{}{
					{
						"ID": "Predicted-Hotspot-Alpha",
						"Lat": 13.02,
						"Lon": 80.21,
						"RadiusKm": 2.5,
					},
				},
			})
		})
	}

	port := ":6081"
	slog.Info("Engine REST API active", "port", port)

	// router.Run(port)

	srv := &http.Server{
		Addr:    port,
		Handler: router,
	}

	// Start server async
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
		}
	}()
	
	//GRACEFUL SHUTDOWN
	// Listen for OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	slog.Warn("Shutdown signal received. Cleaning up...")

	// Create timeout context
	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Stop HTTP server
	if err := srv.Shutdown(ctxShutdown); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	// 2. Stop background workers (IMPORTANT)
	cancel() // main ctx → stops consumers, archiver, rebalancer

	// 3. Close Redis
	if err := redisClient.Close(); err != nil {
		slog.Error("Redis close failed", "error", err)
	}

	slog.Info("Shutdown complete")
}
