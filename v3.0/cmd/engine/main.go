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
// Define the coordinates for the 15 Polaris Chennai zones
// PredictedZone represents a demand hotspot with real-world coordinates
type PredictedZone struct {
	ID       int     `json:"id"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Status   string  `json:"status"`
	RadiusKm float64 `json:"radius_km"`
}
// Match the JSON structure coming from your Python Flask Sidecar
type AIRawResponse struct {
	Status string `json:"status"`
	Data   struct {
		HotZones  []int   `json:"hot_zones"`
		Rebalance [][]int `json:"rebalance"`
	} `json:"data"`
}

// The enriched object used by your Go Engine and React Frontend
type HydratedZone struct {
	ID       int     `json:"id"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Status   string  `json:"status"`
	RadiusKm float64 `json:"radius_km"`
}
// 15 real Chennai zones matching Traffic_15nodes.csv
// 0=T_Nagar, 1=Anna_Nagar, 2=Adyar, 3=OMR, 4=Velachery,
// 5=Mylapore, 6=Guindy, 7=Nungambakkam, 8=Egmore, 9=Perambur,
// 10=Royapettah, 11=Thiruvanmiyur, 12=Porur, 13=Chromepet, 14=Tambaram
var polarisZones = []struct {
	Lat float64
	Lon float64
}{
	{13.0418, 80.2341}, // 0  T_Nagar
	{13.0850, 80.2101}, // 1  Anna_Nagar
	{13.0012, 80.2565}, // 2  Adyar
	{12.9610, 80.2425}, // 3  OMR_Thoraipakkam
	{13.0067, 80.2206}, // 4  Velachery
	{13.0368, 80.2676}, // 5  Mylapore
	{13.0067, 80.2206}, // 6  Guindy
	{13.0569, 80.2425}, // 7  Nungambakkam
	{13.0732, 80.2609}, // 8  Egmore
	{13.1070, 80.2320}, // 9  Perambur
	{13.0500, 80.2600}, // 10 Royapettah
	{12.9830, 80.2594}, // 11 Thiruvanmiyur
	{13.0382, 80.1574}, // 12 Porur
	{12.9516, 80.1462}, // 13 Chromepet
	{12.9249, 80.1378}, // 14 Tambaram
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

	archiver, err := stream.NewPostgresArchiver(redisURL, postgresURL)
	if err != nil {
        slog.Warn("Failed to init Postgres Archiver (DB might not be ready). Archiver disabled for this run.", "error", err)
        // By catching this, the engine will stay alive even if the DB is slow to boot.
    } else {
        go archiver.Start(ctx)
    }

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop() 
		for {
			select {
			case <-ticker.C:
				// Use the new helper here!
				payload, err := FetchAndHydrateAI()
				if err == nil {
					// updatePredictedHeatmapCache now receives []HydratedZone
					updatePredictedHeatmapCache(payload)
				} else {
					slog.Error("Background AI fetch failed", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

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
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")

		// 1. Fetch from Python AI
		resp, err := http.Get("http://gnn-sidecar:5050/demand/forecast")
		if err != nil {
			slog.Error("Network error reaching AI sidecar", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI sidecar unreachable"})
			return
		}
		defer resp.Body.Close()

		// 2. SAFETY CHECK: If Python sent a 500 error, don't try to parse it!
		if resp.StatusCode != http.StatusOK {
			slog.Error("AI Sidecar returned an error", "status", resp.Status)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "AI Sidecar internal crash (check Python logs)"})
			return
		}

		// 3. Decode JSON
		var aiRaw struct {
			Status string `json:"status"`
			Data   struct {
				HotZones  []int                    `json:"hot_zones"`
				Rebalance []map[string]interface{} `json:"rebalance"`
			} `json:"data"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&aiRaw); err != nil {
			slog.Error("JSON Decode Failed", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "AI sent invalid data format"})
			return
		}

		// 4. Hydrate with coordinates
		var hydratedData []PredictedZone
		for _, zoneIdx := range aiRaw.Data.HotZones {
			// Ensure polarisZones is defined and zoneIdx is within range
			if zoneIdx >= 0 && zoneIdx < len(polarisZones) {
				coords := polarisZones[zoneIdx]
				hydratedData = append(hydratedData, PredictedZone{
					ID:       zoneIdx,
					Lat:      coords.Lat,
					Lon:      coords.Lon,
					Status:   "SPIKE_PREDICTED",
					RadiusKm: 2.0,
				})
			}
		}

		// 5. Success
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   hydratedData,
			"meta":   aiRaw.Data.Rebalance,
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
func updatePredictedHeatmapCache(data interface{}) {
    slog.Info("Heatmap cache updated from AI sidecar")
    // You can implement the actual memory caching logic here later!
}
func FetchAndHydrateAI() ([]HydratedZone, error) {
	// 1. Call the Python Sidecar
	resp, err := http.Get("http://gnn-sidecar:5050/demand/forecast")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 2. Decode the raw AI response
	var aiRaw AIRawResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiRaw); err != nil {
		return nil, err
	}

	// 3. Hydrate the data using polarisZones (from your previous main.go update)
	var hydratedData []HydratedZone
	for _, zoneIdx := range aiRaw.Data.HotZones {
		// Safety check: ensure index is within our known zones
		if zoneIdx >= 0 && zoneIdx < len(polarisZones) {
			coords := polarisZones[zoneIdx]
			hydratedData = append(hydratedData, HydratedZone{
				ID:       zoneIdx,
				Lat:      coords.Lat,
				Lon:      coords.Lon,
				Status:   "CRITICAL_DEMAND",
				RadiusKm: 2.0,
			})
		}
	}

	return hydratedData, nil
}