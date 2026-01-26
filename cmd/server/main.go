// package main

// import (
// 	"context"
// 	"log/slog"
// 	"os"
// 	"os/signal"
// 	"syscall"
// 	"time"

// 	"github.com/Akashpg-M/polaris/internal/adapter/engine"
// 	"github.com/Akashpg-M/polaris/internal/config"
// 	"github.com/Akashpg-M/polaris/internal/core/entity"
// 	"github.com/Akashpg-M/polaris/internal/core/ports"
// )

// func main() {
// 	cfg := config.Load()
// 	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
// 	logger.Info("starting polaris engine", "map_width", cfg.MapWidth)

// 	var matcher ports.MatchingEngine = engine.NewInMemoryEngine(cfg.MapWidth, cfg.MapHeight, logger)

// 	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
// 	defer stop()

// 	updateChan := make(chan entity.LocationUpdate, 1000)

// 	// Worker
// 	go func() {
// 		logger.Info("ingestion worker started")
// 		for {
// 			select {
// 			case update := <-updateChan:
// 				matcher.UpdateDriverLocation(update)
// 			case <-ctx.Done():
// 				return
// 			}
// 		}
// 	}()

// 	// Simulation Data
// 	go func() {
// 		logger.Info("simulating traffic")
// 		updates := []entity.LocationUpdate{
// 			{DriverID: "D1", Lat: 10, Lon: 10},
// 			{DriverID: "D2", Lat: 12, Lon: 12},
// 			{DriverID: "D3", Lat: 80, Lon: 80},
// 		}
// 		for _, u := range updates {
// 			updateChan <- u
// 		}
// 	}()

// 	// Main Loop
// 	ticker := time.NewTicker(2 * time.Second)
// 	defer ticker.Stop()

// 	run := true
// 	for run {
// 		select {
// 		case <-ticker.C:
// 			matches, _ := matcher.FindNearestDrivers(10, 10, 2)
// 			logger.Info("search result", "found", len(matches))
// 		case <-ctx.Done():
// 			logger.Info("shutdown signal received")
// 			run = false
// 		}
// 	}
// 	logger.Info("polaris exited gracefully")
// }

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors" // CORS lib
	"github.com/gin-gonic/gin"    // Gin Framework

	"github.com/Akashpg-M/polaris/internal/adapter/engine"
	"github.com/Akashpg-M/polaris/internal/adapter/handler"
	"github.com/Akashpg-M/polaris/internal/config"
	"github.com/Akashpg-M/polaris/internal/core/ports"
)

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// In production, replace "*" with your specific domain
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		
		// Handle "Preflight" requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}
func main() {
	// 1. Setup
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("starting polaris engine", "version", "v0.3-gin", "env", "dev")

	// Set Gin to Release Mode in Production to silence debug logs
	if cfg.LogLevel != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 2. Initialize Engine
	var matcher ports.MatchingEngine = engine.NewInMemoryEngine(cfg.MapWidth, cfg.MapHeight, logger)

	// 3. Initialize Handler
	httpHandler := handler.NewHTTPHandler(matcher)

	// 4. Setup Gin Router
	router := gin.Default() // Includes Logger and Recovery middleware automatically

	// 5. Configure CORS (Production Grade)
	// allow all (*) for dev, but in prod you restrict this to domain
	corsConfig := cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	router.Use(cors.New(corsConfig))

	// 6. Define Routes
	router.POST("/driver/location", httpHandler.UpdateLocation)
	router.GET("/ride/match", httpHandler.FindMatches)
	router.GET("/ws/driver", httpHandler.DriverSocket)
	router.POST("/ride/book", httpHandler.BookRide) 
	
	// 7. Start Server (With Graceful Shutdown)
	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	go func() {
		logger.Info("gin server listening on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server startup failed", "error", err)
			os.Exit(1)
		}
	}()

	// 8. Graceful Shutdown Signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("forced shutdown", "error", err)
	}
	logger.Info("server exited gracefully")
}