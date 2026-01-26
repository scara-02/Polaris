package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Akashpg-M/polaris/internal/adapter/engine"
	"github.com/Akashpg-M/polaris/internal/config"
	"github.com/Akashpg-M/polaris/internal/core/entity"
	"github.com/Akashpg-M/polaris/internal/core/ports"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	logger.Info("starting polaris engine", "map_width", cfg.MapWidth)

	var matcher ports.MatchingEngine = engine.NewInMemoryEngine(cfg.MapWidth, cfg.MapHeight, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	updateChan := make(chan entity.LocationUpdate, 1000)

	// Worker
	go func() {
		logger.Info("ingestion worker started")
		for {
			select {
			case update := <-updateChan:
				matcher.UpdateDriverLocation(update)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Simulation Data
	go func() {
		logger.Info("simulating traffic")
		updates := []entity.LocationUpdate{
			{DriverID: "D1", Lat: 10, Lon: 10},
			{DriverID: "D2", Lat: 12, Lon: 12},
			{DriverID: "D3", Lat: 80, Lon: 80},
		}
		for _, u := range updates {
			updateChan <- u
		}
	}()

	// Main Loop
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	run := true
	for run {
		select {
		case <-ticker.C:
			matches, _ := matcher.FindNearestDrivers(10, 10, 2)
			logger.Info("search result", "found", len(matches))
		case <-ctx.Done():
			logger.Info("shutdown signal received")
			run = false
		}
	}
	logger.Info("polaris exited gracefully")
}