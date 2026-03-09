package orchestrator

import (
	"context"
	"log/slog"
	"time"

	"github.com/Akashpg-M/polaris/internal/application/spatial"
	"github.com/Akashpg-M/polaris/internal/core/domain"
	"github.com/Akashpg-M/polaris/internal/core/ports"
)

type CommandPayload struct {
	Directive string  `json:"directive"`
	TargetLat float64 `json:"target_lat"`
	TargetLon float64 `json:"target_lon"`
}

type Rebalancer struct {
	engine    *spatial.Engine
	commander ports.FleetCommander
}

func NewRebalancer(engine *spatial.Engine, commander ports.FleetCommander) *Rebalancer {
	return &Rebalancer{engine: engine, commander: commander}
}

func (r *Rebalancer) StartAutonomousLoop(ctx context.Context) {
	slog.Info("Autonomous Rebalancing Engine Activated")
	
	// Wake up every 15 seconds to evaluate the grid
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Define a "Cold Zone" (e.g., Guindy area in Chennai)
	targetLat := 13.0067
	targetLon := 80.2206
	coverageRadius := 5.0 // km

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 1. Scan the Cold Zone to see how many drones are currently there
			nodesInZone := r.engine.FindNearest(targetLat, targetLon, coverageRadius, uint16(domain.ClassDrone))
			
			// 2. Decision Logic: We want at least 3 drones in this zone
			deficit := 3 - len(nodesInZone)
			
			if deficit > 0 {
				slog.Warn("Cold Zone Detected! Executing Autonomous Relocation.", "deficit", deficit)
				
				// 3. Find the nearest drones from a much wider radius (e.g., 20km) to pull them in
				availableNodes := r.engine.FindNearest(targetLat, targetLon, 20.0, uint16(domain.ClassDrone))
				
				dispatched := 0
				for _, node := range availableNodes {
					if dispatched >= deficit {
						break
					}
					
					// Ignore nodes that are already inside the cold zone
					if node.DistanceKm < coverageRadius {
						continue
					}

					// 4. Push the RELOCATE command down the WebSocket to the specific physical device
					cmd := CommandPayload{
						Directive: "RELOCATE",
						TargetLat: targetLat,
						TargetLon: targetLon,
					}
					
					err := r.commander.SendCommand(node.NodeID, cmd)
					if err == nil {
						slog.Info("Relocation Directive Transmitted", "node", node.NodeID, "target", "Guindy")
						dispatched++
					}
				}
			}
		}
	}
}