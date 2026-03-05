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
	"github.com/Akashpg-M/polaris/internal/adapter/repository"
	"github.com/Akashpg-M/polaris/internal/adapter/osrm"
)


func main() {
	// 1. Load Config (From .env or Environment)
	// cfg := config.Load()

	// // 2. Setup Logger
	// logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// logger.Info("starting polaris engine", 
	// 	"version", "v0.6-env", 
	// 	"env", cfg.AppEnv, 
	// 	"port", cfg.Port,
	// )
	cfg := config.Load()
	osrmClient := osrm.NewClient(cfg.OSRMUrl)


	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Info("starting polaris engine", 
		"version", "v0.6-env", 
		"env", cfg.AppEnv, 
		"port", cfg.Port,
	)
	
	// 1. Connect to Postgres
	logger.Info("connecting to database...", "url", cfg.DBUrl)
	pgRepo, err := repository.NewPostgresRepo(cfg.DBUrl)
	if err != nil {
			logger.Error("postgres init failed", "error", err)
			os.Exit(1)
	}

	// 2. Connect to Redis (NEW)
	logger.Info("connecting to redis...", "url", cfg.RedisUrl)
	redisRepo, err := repository.NewRedisRepo(cfg.RedisUrl)
	if err != nil {
			logger.Error("redis init failed", "error", err)
			os.Exit(1)
	}

	// 3. Init Engine with BOTH Repos
	var matcher ports.MatchingEngine = engine.NewInMemoryEngine(
			cfg.MapWidth, 
			cfg.MapHeight, 
			logger, 
			pgRepo,    // For Hydration & Booking
			redisRepo, // For Location & Locks
			osrmClient,
	)

	// Set Gin Mode based on Config
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

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


// Flow Visualization:

// main()
//  ├── Create PostgresRepo
//  ├── Create RedisRepo
//  ├── Create InMemoryEngine (concrete struct)
//  ├── Store it inside interface variable (matcher)
//  ├── Inject matcher into handler
//  ├── Start HTTP server

// Request comes in
//     ↓
// Handler calls engine interface
//     ↓
// Go dispatches to InMemoryEngine method
//     ↓
// Redis / Postgres get used internally


// Issue:
// If tomorrow you deploy:
// 3 Polaris servers
// Each has its own:
// InMemoryEngine.drivers map
// What happens if:
// Server A updates driver location
// Server B receives ride booking
// Will they see the same memory state?


// What is the current project level:
// system architecture that mirrors what Uber uses for their "Hot Path."
// Locations: High-Frequency -> Redis
// Transactions: High-Value -> Postgres
// Search: High-Speed -> QuadTree (RAM)

// GPS Update → Geo Index → Match → Lock → Confirm


//Next Step:
// Solve for distibuted system, how can we make it distributed
// Currently:
//   Geo index is local memory
//   Drivers map is local memory
//   Only locks are distributed
// So works well for Single Instance but not for Multi instance cluster