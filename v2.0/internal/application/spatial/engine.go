package spatial

import (
	"log/slog"
	"sync"
	"time"
	"sort"

	"github.com/Akashpg-M/polaris/internal/core/domain"
	"github.com/Akashpg-M/polaris/algo_/quadtree"
	"github.com/Akashpg-M/polaris/algo_/geo"

)

type Engine struct {
	mu    sync.RWMutex
	nodes map[string]*domain.TelemetryPayload
	qt    *quadtree.SafeQuadTree
}

// MatchResult is the DTO (Data Transfer Object) sent back to the dispatcher
type MatchResult struct {
	NodeID     string  `json:"node_id"`
	Class      uint16  `json:"asset_class"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	DistanceKm float64 `json:"distance_km"`
	ETASec     int     `json:"eta_seconds"`
	RouteType  string  `json:"route_type"`
}

func NewEngine() *Engine {
	// Initialize a bounding box large enough to cover the Earth (or specifically Chennai)
	bounds := quadtree.Bounds{
		X:      12.0, // Min Latitude (South bound)
		Y:      79.5, // Min Longitude (West bound)
		Width:  2.0,  // Spans up to Latitude 14.0 (North bound)
		Height: 1.5,  // Spans up to Longitude 81.0 (East bound)
	}	
	return &Engine{
		nodes: make(map[string]*domain.TelemetryPayload),
		qt:    quadtree.NewSafeQuadTree(bounds),
	}
}

// BatchUpdate processes thousands of pings with a single lock
// func (e *Engine) BatchUpdate(payloads []domain.TelemetryPayload) {
// 	if len(payloads) == 0 {
// 		return
// 	}

// 	e.mu.Lock()
// 	defer e.mu.Unlock()

// 	start := time.Now()

// 	for _, payload := range payloads {
// 		// 1. Update the RAM Map
// 		// Create a copy of the payload to store securely in memory
// 		p := payload 
// 		e.nodes[p.NodeID] = &p

// 		// 2. Update the QuadTree Index
// 		e.qt.Insert(quadtree.Point{
// 			Lat:   p.Lat,
// 			Lon:   p.Lon,
// 			ID:    p.NodeID,
// 			Class: uint16(p.Class),
// 			TenantID: p.TenantID,
// 		})
// 	}

// 	slog.Debug("Batch update complete", "processed", len(payloads), "duration_ms", time.Since(start).Milliseconds())
// }

// BatchUpdate processes thousands of pings with a single lock
func (e *Engine) BatchUpdate(payloads []domain.TelemetryPayload) {
	if len(payloads) == 0 {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	start := time.Now()

	for _, payload := range payloads {
		p := payload 

		// 1. Check if we already have a previous location for this NodeID
		if _, exists := e.nodes[p.NodeID]; exists {
			// Delete the old coordinate from the QuadTree to prevent "ghosts"
			e.qt.Remove(p.NodeID)
		}

		// 2. Update the RAM Map with the fresh payload
		e.nodes[p.NodeID] = &p

		// 3. Insert the fresh coordinate into the QuadTree
		e.qt.Insert(quadtree.Point{
			Lat:   p.Lat,
			Lon:   p.Lon,
			ID:    p.NodeID,
			Class: uint16(p.Class),
		})
	}

	slog.Debug("Batch update complete", "processed", len(payloads), "duration_ms", time.Since(start).Milliseconds())
}

// FindNearest queries the QuadTree, filters by exact distance, and applies context-aware routing.
func (e *Engine) FindNearest(tenantID string, lat, lon, radiusKm float64, reqClass uint16) []MatchResult {
	// 1. Calculate the search boundary
	x, y, w, h := geo.BoundingBox(lat, lon, radiusKm)
	searchBounds := quadtree.Bounds{X: x, Y: y, Width: w, Height: h}

	// 2. Hardware-level filter: Query the QuadTree ($O(\log n)$)
	candidates := e.qt.Search(searchBounds, reqClass)

	var results []MatchResult

	// 3. Refine candidates with exact Earth curvature math and Context-Aware Routing
	e.mu.RLock()
	for _, c := range candidates {
		if c.TenantID != tenantID {
				continue
		}

		dist := geo.Haversine(lat, lon, c.Lat, c.Lon)
		
		// The QuadTree returns a square box; we filter out corners to make a perfect circle
		if dist <= radiusKm {
			// Context-Aware Routing Logic
			var eta int
			var routeType string

			if (c.Class & uint16(domain.ClassDrone)) > 0 {
				// Drones fly point-to-point. Assume 60 km/h (1 km per minute)
				eta = int((dist / 60.0) * 3600)
				routeType = "euclidean_air"
			} else {
				// Ground Vehicles use street networks. 
				// For now, we simulate a Manhattan distance penalty (x1.4) at 40 km/h.
				// In production, this candidate would be sent to your OSRM container.
				streetDist := dist * 1.4 
				eta = int((streetDist / 40.0) * 3600)
				routeType = "osrm_street"
			}

			results = append(results, MatchResult{
				NodeID:     c.ID,
				Class:      c.Class,
				Lat:        c.Lat,
				Lon:        c.Lon,
				DistanceKm: dist,
				ETASec:     eta,
				RouteType:  routeType,
			})
		}
	}
	e.mu.RUnlock()

	// 4. Sort results by ETA (fastest arrival first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].ETASec < results[j].ETASec
	})

	// Limit to top 50 matches to save bandwidth
	if len(results) > 50 {
		return results[:50]
	}
	return results
}