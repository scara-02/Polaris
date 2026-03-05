package engine

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/Akashpg-M/polaris/internal/adapter/repository"
	"github.com/Akashpg-M/polaris/internal/core/entity"
	"github.com/Akashpg-M/polaris/pkg/quadtree"
	"github.com/Akashpg-M/polaris/internal/adapter/osrm"
)

type InMemoryEngine struct {
	qt        *quadtree.SafeQuadTree
	drivers   map[string]*entity.Driver
	mu        sync.RWMutex
	logger    *slog.Logger
	pgRepo    *repository.PostgresRepo // Permanent Storage
	redisRepo *repository.RedisRepo    // Fast Buffer & Locks
	osrmClient *osrm.Client
}

func NewInMemoryEngine(
	width, height float64, 
	logger *slog.Logger, 
	pg *repository.PostgresRepo, 
	rdb *repository.RedisRepo,
	osrmClient *osrm.Client,
) *InMemoryEngine {
	e := &InMemoryEngine{
		qt:        quadtree.NewSafeQuadTree(quadtree.Bounds{X: 0, Y: 0, Width: width, Height: height}),
		drivers:   make(map[string]*entity.Driver),
		logger:    logger,
		pgRepo:    pg,
		redisRepo: rdb,
		osrmClient: osrmClient,
	}

	// 1. Restore State from DB
	e.hydrate()
	// 2. Start Background Janitor (Cleanup Stale Drivers)
	go e.runJanitor()

	return e
}

// runJanitor removes drivers who haven't updated in 2 minutes
func (e *InMemoryEngine) runJanitor() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		e.mu.Lock()
		now := time.Now()
		deleted := 0

		for id, driver := range e.drivers {
			// If last seen > 2 mins ago
			if now.Sub(driver.UpdatedAt) > 2*time.Minute {
				delete(e.drivers, id)
				// Note: Removing from QuadTree is harder without rebuilding,
				// but since we filter by ID in the Search function,
				// deleting from the map effectively "hides" them.
				// This is acceptable short-term, But long-running servers will accumulate dead points
				// Industry fix (later):
				// 	Periodic QuadTree rebuild
				// 	Or use lazy deletion flag
				// 	Or shard QuadTrees by time window
				deleted++
			}
		}
		e.mu.Unlock()

		if deleted > 0 {
			e.logger.Info("janitor cleanup", "removed_drivers", deleted)
		}
	}
}

func (e *InMemoryEngine) UpdateDriverLocation(update entity.LocationUpdate) error {
	e.mu.Lock()
	defer e.mu.Unlock() // This handles the lock for the entire function

	// 1. Update Memory (RAM)
	driver, exists := e.drivers[update.DriverID]
	if !exists {
		driver = &entity.Driver{ID: update.DriverID, Status: entity.DriverAvailable}
		e.drivers[update.DriverID] = driver
	}
	driver.Lat = update.Lat
	driver.Lon = update.Lon
	driver.UpdatedAt = time.Now()

	// 2. Update QuadTree (RAM Index)
	e.qt.Insert(quadtree.Point{Lat: update.Lat, Lon: update.Lon, Data: update.DriverID})

	// 3. Update Redis (Write-Behind Buffer)
	// Pass the values directly into the goroutine
	go func(id string, lat, lon float64) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := e.redisRepo.UpdateLocation(ctx, id, lat, lon); err != nil {
			e.logger.Error("failed to push location to redis", "error", err)
		}
	}(update.DriverID, update.Lat, update.Lon)

	return nil
}

func (e *InMemoryEngine) BookDriver(driverID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Distributed Lock (Redis)
	// "Hey Redis, I am booking Driver X. Stop anyone else."
	acquired, err := e.redisRepo.AcquireLock(ctx, driverID)
	if err != nil {
		return fmt.Errorf("redis error: %w", err)
	}
	if !acquired {
		return fmt.Errorf("driver is currently being booked by someone else")
	}
	// Always release lock when done (even if error happens later)
	defer e.redisRepo.ReleaseLock(ctx, driverID)

	// 2. Read Memory (SHORT lock)
	e.mu.RLock()
	driver, exists := e.drivers[driverID]
	if !exists || driver.Status != entity.DriverAvailable {
		e.mu.RUnlock()
		return fmt.Errorf("driver unavailable")
	}

	lat := driver.Lat
	lon := driver.Lon
	e.mu.RUnlock()

	// 3. Persist to Postgres (NO lock)
	if err := e.pgRepo.SaveDriver(driverID, lat, lon, string(entity.DriverBooked)); err != nil {
		return fmt.Errorf("db save failed: %w", err)
	}

	if err := e.pgRepo.CreateTrip(driverID, lat, lon); err != nil {
		e.logger.Error("trip recording failed", "error", err)
	}

	// 4. Update Memory (SHORT lock)
	e.mu.Lock()
	if d, ok := e.drivers[driverID]; ok {
		d.Status = entity.DriverBooked
	}
	e.mu.Unlock()

	e.logger.Info("driver booked", "id", driverID)
	return nil
}

// Helper: Hydrate
func (e *InMemoryEngine) hydrate() {
	e.logger.Info("hydrating from postgres...")
	drivers, err := e.pgRepo.FetchActiveDrivers(60)
	if err != nil {
		e.logger.Error("hydration failed", "error", err)
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, d := range drivers {
		e.drivers[d.ID] = &d
		e.qt.Insert(quadtree.Point{Lat: d.Lat, Lon: d.Lon, Data: d.ID})
	}
}

// // we do an general Delete-then-Insert Approach - but in real time systems this is costly
// func (e *InMemoryEngine) FindNearestDrivers(lat, lon float64, k int) ([]entity.Driver, error) {
// 	e.logger.Debug("executing spatial search", "lat", lat, "lon", lon)

// 	searchRadius := 50.0
// 	bounds := quadtree.Bounds{X: lat - searchRadius, Y: lon - searchRadius, Width: searchRadius * 2, Height: searchRadius * 2}
// 	points := e.qt.Search(bounds)

// 	var candidates []entity.Driver
// 	e.mu.RLock()
// 	for _, p := range points {
// 		driver, exists := e.drivers[p.Data]
// 		if exists && driver.Status == entity.DriverAvailable {
// 			candidates = append(candidates, *driver)
// 		}
// 	}
// 	e.mu.RUnlock()

// 	sort.Slice(candidates, func(i, j int) bool {
// 		return distance(lat, lon, candidates[i].Lat, candidates[i].Lon) < distance(lat, lon, candidates[j].Lat, candidates[j].Lon)
// 	})

// 	if len(candidates) > k {
// 		return candidates[:k], nil
// 	}
// 	return candidates, nil
// }

func (e *InMemoryEngine) FindNearestDrivers(lat, lon float64, k int) ([]entity.Driver, error) {
	e.logger.Debug("executing two-stage spatial search", "lat", lat, "lon", lon)

	// STAGE 1: THE FILTER (RAM)
	// Find top 50 straight-line candidates to avoid overloading OSRM
	searchRadius := 50.0
	bounds := quadtree.Bounds{X: lat - searchRadius, Y: lon - searchRadius, Width: searchRadius * 2, Height: searchRadius * 2}
	points := e.qt.Search(bounds)

	var candidates []entity.Driver
	e.mu.RLock()
	for _, p := range points {
		driver, exists := e.drivers[p.Data]
		if exists && driver.Status == entity.DriverAvailable {
			candidates = append(candidates, *driver)
		}
	}
	e.mu.RUnlock()

	// Sort by straight-line distance and slice to top 50
	sort.Slice(candidates, func(i, j int) bool {
		return distance(lat, lon, candidates[i].Lat, candidates[i].Lon) < distance(lat, lon, candidates[j].Lat, candidates[j].Lon)
	})
	
	limit := 50
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	if len(candidates) == 0 {
		return candidates, nil
	}

	// STAGE 2: THE REFINER (OSRM)
	// Get real-world driving times
	refinedCandidates, err := e.osrmClient.CalculateETAs(lat, lon, candidates)
	if err != nil {
		e.logger.Error("osrm failed, falling back to straight-line", "error", err)
		// Fallback: Return straight-line results if OSRM crashes
		if len(candidates) > k {
			return candidates[:k], nil
		}
		return candidates, nil
	}

	// STAGE 3: THE SORTER
	// Sort by ETA (Time) instead of straight-line distance
	sort.Slice(refinedCandidates, func(i, j int) bool {
		return refinedCandidates[i].ETA < refinedCandidates[j].ETA
	})

	// Return top K
	if len(refinedCandidates) > k {
		return refinedCandidates[:k], nil
	}
	return refinedCandidates, nil
}


func distance(x1, y1, x2, y2 float64) float64 {
	return math.Sqrt(math.Pow(x1-x2, 2) + math.Pow(y1-y2, 2))
}

//further optimization possiblity

// "Remove and Insert" is slightly expensive because the tree constantly re-balances.
// In Real-world engines often use:
// 1. Lazy Deletion: Mark the old point as "dead" (boolean flag) instead of actually removing it from memory array, then clean up in batches later.
// 2. Moving Objects Tree: A specialized version of a QuadTree designed specifically for things that move.

// NewInMemoryEngine()
//  ├─ hydrate()        → restore active drivers from Postgres
//  ├─ runJanitor()     → cleanup dead drivers every minute

// Driver moves
//  ├─ Update map (RAM)
//  ├─ Insert into QuadTree
//  ├─ Push update to Redis (async)

// Client books driver
//  ├─ Acquire Redis lock (distributed safety)
//  ├─ Check memory availability
//  ├─ Write booking to Postgres (truth)
//  ├─ Record trip
//  ├─ Update memory

// Every 1 minute:
//  ├─ Remove drivers inactive > 2 mins
//  └─ QuadTree cleanup deferred
