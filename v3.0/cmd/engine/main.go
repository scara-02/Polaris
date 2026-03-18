package main

import (
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

// func main() {
// 	logger.Init()
// 	slog.Info("Booting Polaris v3.0 Spatial Engine...")

// 	engine := spatial.NewEngine()
// 	redisURL := "redis://localhost:6379/0"
// 	postgresURL := "postgres://polaris_user:polaris_password@localhost:5432/polaris_core?sslmode=disable"
// 	ctx := context.Background()

// 	// 1. Consumers & Archivers
// 	redisConsumer, _ := stream.NewRedisConsumer(redisURL, engine)
// 	go redisConsumer.Start(ctx, "engine-node-1")

// 	archiver, _ := stream.NewPostgresArchiver(redisURL, postgresURL)
// 	go archiver.Start(ctx)

// 	// 2. Orchestrator with the new RedisCommander
// 	opts, _ := redis.ParseURL(redisURL)
// 	redisClient := redis.NewClient(opts)
// 	commander := &RedisCommander{client: redisClient}
	
// 	demandStrategy := &orchestrator.StaticZoneStrategy{}
// 	rebalancer := orchestrator.NewRebalancer(engine, commander, demandStrategy)
// 	go rebalancer.StartAutonomousLoop(ctx)
	

// 	// 3. Setup Router for REST API (Running on 6081)
// 	gin.SetMode(gin.ReleaseMode)
// 	router := gin.New()
// 	router.Use(gin.Recovery())
// 	router.Use(cors.Default())

// 	matchHandler := handler.NewMatchHandler(engine)
// 	api := router.Group("/api/v1")
// 	{
// 		api.GET("/nodes/match", matchHandler.GetNearestNodes)
// 	}

	

// 	port := ":6081" // Notice the new port!
// 	slog.Info("Engine REST API active", "port", port)
// 	router.Run(port)
// }


func main() {
	logger.Init()
	slog.Info("Booting Polaris v3.0 Spatial Engine...")

	engine := spatial.NewEngine()
	redisURL := "redis://localhost:6379/0"
	postgresURL := "postgres://polaris_user:polaris_password@localhost:5432/polaris_core?sslmode=disable"
	ctx := context.Background()

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
			zones := predictiveStrategy.GetTargetZones(context.Background())
			c.JSON(200, gin.H{
				"status": "success",
				"data":   zones,
			})
		})
	}

	port := ":6081"
	slog.Info("Engine REST API active", "port", port)
	router.Run(port)
}
