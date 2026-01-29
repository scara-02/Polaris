package engine

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/Akashpg-M/polaris/internal/core/entity"
	"github.com/Akashpg-M/polaris/pkg/quadtree"
)

type InMemoryEngine struct {
	qt      *quadtree.SafeQuadTree
	drivers map[string]*entity.Driver
	mu      sync.RWMutex
	logger  *slog.Logger
}

func NewInMemoryEngine(width, height float64, logger *slog.Logger) *InMemoryEngine {
	return &InMemoryEngine{
		qt:      quadtree.NewSafeQuadTree(quadtree.Bounds{X: 0, Y: 0, Width: width, Height: height}),
		drivers: make(map[string]*entity.Driver),
		logger:  logger,
	}
}


// we keep adding new points to the QuadTree without removing the old ones. 
// In a real production app, this would be a memory leak, but for this learning project, 
// it keeps the logic simple
func (e *InMemoryEngine) UpdateDriverLocation(update entity.LocationUpdate) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	driver, exists := e.drivers[update.DriverID]
	if exists {
        oldPoint := quadtree.Point{
            Lat:  driver.Lat,
            Lon:  driver.Lon,
            Data: driver.ID,
        }
        
        e.qt.Remove(oldPoint)
    } else {
        e.logger.Debug("registering new driver", "driver_id", update.DriverID)
        driver = &entity.Driver{ID: update.DriverID, Status: entity.DriverAvailable}
        e.drivers[update.DriverID] = driver
    }
	driver.Lat = update.Lat
	driver.Lon = update.Lon
	driver.UpdatedAt = time.Now()

	e.qt.Insert(quadtree.Point{Lat: update.Lat, Lon: update.Lon, Data: update.DriverID})
	return nil
}

// we do an general Delete-then-Insert Approach - but in real time systems this is costly
func (e *InMemoryEngine) FindNearestDrivers(lat, lon float64, k int) ([]entity.Driver, error) {
	e.logger.Debug("executing spatial search", "lat", lat, "lon", lon)

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

	sort.Slice(candidates, func(i, j int) bool {
		return distance(lat, lon, candidates[i].Lat, candidates[i].Lon) < distance(lat, lon, candidates[j].Lat, candidates[j].Lon)
	})

	if len(candidates) > k {
		return candidates[:k], nil
	}
	return candidates, nil
}


func (e *InMemoryEngine) BookDriver(driverID string) error {
	e.mu.Lock()        
	defer e.mu.Unlock() 
	driver, exists := e.drivers[driverID]
	if !exists {
		return fmt.Errorf("driver not found")
	}

	if driver.Status != entity.DriverAvailable {
		return fmt.Errorf("driver is already booked or offline")
	}

	driver.Status = entity.DriverBooked
	e.logger.Info("driver booked successfully", "driver_id", driverID)
	
	return nil
}

func distance(x1, y1, x2, y2 float64) float64 {
	return math.Sqrt(math.Pow(x1-x2, 2) + math.Pow(y1-y2, 2))
}



//further optimization possiblity

// "Remove and Insert" is slightly expensive because the tree constantly re-balances. 
// In Real-world engines often use:
// 1. Lazy Deletion: Mark the old point as "dead" (boolean flag) instead of actually removing it from memory array, then clean up in batches later.
// 2. Moving Objects Tree: A specialized version of a QuadTree designed specifically for things that move.