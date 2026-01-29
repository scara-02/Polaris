package main

import (
	"context"
	"fmt" // Used for formatting the port string
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/Akashpg-M/polaris/internal/adapter/engine"
	"github.com/Akashpg-M/polaris/internal/adapter/handler"
	"github.com/Akashpg-M/polaris/internal/config"
	"github.com/Akashpg-M/polaris/internal/core/ports"
)

func main() {
	// 1. Load Config (From .env or Environment)
	cfg := config.Load()

	// 2. Setup Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("starting polaris engine", 
		"version", "v0.6-env", 
		"env", cfg.AppEnv, 
		"port", cfg.Port,
	)

	// Set Gin Mode based on Config
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 3. Initialize Engine
	var matcher ports.MatchingEngine = engine.NewInMemoryEngine(cfg.MapWidth, cfg.MapHeight, logger)

	// 4. Initialize Handler
	httpHandler := handler.NewHTTPHandler(matcher)

	// 5. Setup Router
	router := gin.New() // Use New() to manually add middleware
	router.Use(gin.Recovery())
	
	// Add Logger Middleware (Skip logging health checks in prod if needed)
	if cfg.AppEnv != "test" {
		router.Use(gin.Logger())
	}

	// 6. Configure CORS Dynamically
	corsConfig := cors.Config{
		AllowOrigins:     []string{"*"}, // In prod, change this to specific domain from config
		AllowMethods:     []string{"GET", "POST", "PUT", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	router.Use(cors.New(corsConfig))

	// 7. Define Routes
	router.POST("/driver/location", httpHandler.UpdateLocation)
	router.GET("/ride/match", httpHandler.FindMatches)
	router.POST("/ride/book", httpHandler.BookRide)
	router.GET("/ws/driver", httpHandler.DriverSocket)

	// 8. Start Server using Config Port
	srv := &http.Server{
		Addr:    ":" + cfg.Port, // Uses value from .env (e.g., ":8080")
		Handler: router,
	}

	go func() {
		logger.Info(fmt.Sprintf("gin server listening on :%s", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server startup failed", "error", err)
			os.Exit(1)
		}
	}()

	// 9. Graceful Shutdown
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