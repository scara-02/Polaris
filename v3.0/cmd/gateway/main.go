package main

import (
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

	redisURL := "redis://localhost:6379/0"
	
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