// package orchestrator

// import (
// 	"context"
// 	"log/slog"
// 	"time"

// 	"github.com/Akashpg-M/polaris/internal/application/spatial"
// 	"github.com/Akashpg-M/polaris/internal/core/domain"
// 	"github.com/Akashpg-M/polaris/internal/core/ports"
// )

// type CommandPayload struct {
// 	Directive string  `json:"directive"`
// 	TargetLat float64 `json:"target_lat"`
// 	TargetLon float64 `json:"target_lon"`
// }

// type Rebalancer struct {
// 	engine    *spatial.Engine
// 	commander ports.FleetCommander
// }

// func NewRebalancer(engine *spatial.Engine, commander ports.FleetCommander) *Rebalancer {
// 	return &Rebalancer{engine: engine, commander: commander}
// }

// func (r *Rebalancer) StartAutonomousLoop(ctx context.Context) {
// 	slog.Info("Autonomous Rebalancing Engine Activated")
	
// 	// Wake up every 15 seconds to evaluate the grid
// 	ticker := time.NewTicker(15 * time.Second)
// 	defer ticker.Stop()

// 	// Define a "Cold Zone" (e.g., Guindy area in Chennai)
// 	targetLat := 13.0067
// 	targetLon := 80.2206
// 	coverageRadius := 5.0 // km

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return
// 		case <-ticker.C:
// 			// 1. Scan the Cold Zone to see how many drones are currently there
// 			nodesInZone := r.engine.FindNearest(targetLat, targetLon, coverageRadius, uint16(domain.ClassDrone))
			
// 			// 2. Decision Logic: We want at least 3 drones in this zone
// 			deficit := 3 - len(nodesInZone)
			
// 			if deficit > 0 {
// 				slog.Warn("Cold Zone Detected! Executing Autonomous Relocation.", "deficit", deficit)
				
// 				// 3. Find the nearest drones from a much wider radius (e.g., 20km) to pull them in
// 				availableNodes := r.engine.FindNearest(targetLat, targetLon, 20.0, uint16(domain.ClassDrone))
				
// 				dispatched := 0
// 				for _, node := range availableNodes {
// 					if dispatched >= deficit {
// 						break
// 					}
					
// 					// Ignore nodes that are already inside the cold zone
// 					if node.DistanceKm < coverageRadius {
// 						continue
// 					}

// 					// 4. Push the RELOCATE command down the WebSocket to the specific physical device
// 					cmd := CommandPayload{
// 						Directive: "RELOCATE",
// 						TargetLat: targetLat,
// 						TargetLon: targetLon,
// 					}
					
// 					err := r.commander.SendCommand(node.NodeID, cmd)
// 					if err == nil {
// 						slog.Info("Relocation Directive Transmitted", "node", node.NodeID, "target", "Guindy")
// 						dispatched++
// 					}
// 				}
// 			}
// 		}
// 	}
// }

package orchestrator

import (
	"context"
	"log/slog"
	"time"

	"github.com/Akashpg-M/polaris/internal/application/spatial"
	"github.com/Akashpg-M/polaris/internal/core/ports"
)

// 1. THE GENERALIZED DOMAIN MODELS

// Zone represents a dynamic geographic area requiring coverage.
type Zone struct {
	ID             string
	Lat            float64
	Lon            float64
	RadiusKm       float64
	RequiredAssets int
	TargetClass    uint16 // e.g., Drone vs Vehicle
    TenantID       string // Supports SaaS isolation
}

// CommandPayload is the JSON sent to the physical device
type CommandPayload struct {
	Directive string  `json:"directive"`
	TargetLat float64 `json:"target_lat"`
	TargetLon float64 `json:"target_lon"`
}

// 2. THE STRATEGY INTERFACE

// DemandStrategy allows you to inject different business logic (Static Hubs, Live Heatmaps, Machine Learning predictions) 
// without ever rewriting the Rebalancer's core routing logic.
type DemandStrategy interface {
	GetTargetZones(ctx context.Context) []Zone
}

// 3. THE ORCHESTRATOR

type Rebalancer struct {
	engine    *spatial.Engine
	commander ports.FleetCommander
	strategy  DemandStrategy // Injected policy
}

func NewRebalancer(engine *spatial.Engine, commander ports.FleetCommander, strategy DemandStrategy) *Rebalancer {
	return &Rebalancer{
		engine:    engine,
		commander: commander,
		strategy:  strategy,
	}
}

func (r *Rebalancer) StartAutonomousLoop(ctx context.Context) {
	slog.Info("Generalized Autonomous Rebalancing Engine Activated")
	
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 1. Fetch dynamic zones from the injected Strategy
			zones := r.strategy.GetTargetZones(ctx)

			// 2. Evaluate and rebalance each zone independently
			for _, zone := range zones {
				r.processZone(zone)
			}
		}
	}
}

// processZone handles the mathematical routing for a single geographic deficit
func (r *Rebalancer) processZone(zone Zone) {
	// 1. Scan the zone to see current supply
	// Note: In a fully production system, FindNearest should be updated to only return nodes with Status == 'idle'
	nodesInZone := r.engine.FindNearest(zone.TenantID, zone.Lat, zone.Lon, zone.RadiusKm, zone.TargetClass)
	
	deficit := zone.RequiredAssets - len(nodesInZone)
	
	if deficit > 0 {
		slog.Warn("Zone Deficit Detected! Executing Relocation.", 
			"zone", zone.ID, 
			"deficit", deficit,
			"target_class", zone.TargetClass)
		
		// 2. Scan a wider area (e.g., 25km) to find idle assets to pull in
		availableNodes := r.engine.FindNearest(zone.TenantID, zone.Lat, zone.Lon, 25.0, zone.TargetClass)
		
		dispatched := 0
		for _, node := range availableNodes {
			if dispatched >= deficit {
				break
			}
			
			// Ignore nodes that are already physically inside the target zone
			if node.DistanceKm <= zone.RadiusKm {
				continue
			}

			// 3. Push the directive down the WebSocket to the physical hardware
			cmd := CommandPayload{
				Directive: "RELOCATE",
				TargetLat: zone.Lat,
				TargetLon: zone.Lon,
			}
			
			err := r.commander.SendCommand(node.NodeID, cmd)
			if err == nil {
				slog.Info("Relocation Directive Transmitted", "node", node.NodeID, "target_zone", zone.ID)
				dispatched++
			}
		}
	}
}