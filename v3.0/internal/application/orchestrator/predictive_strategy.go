package orchestrator

import (
	"fmt"
	"context"
	"log/slog"
	"github.com/jmoiron/sqlx"
	"github.com/Akashpg-M/polaris/internal/core/domain"
)

// PredictiveZoneStrategy uses historical spatial clustering to predict demand
type PredictiveZoneStrategy struct {
	db *sqlx.DB
}

func NewPredictiveZoneStrategy(postgresURL string) (*PredictiveZoneStrategy, error) {
	db, err := sqlx.Connect("postgres", postgresURL)
	if err != nil {
		return nil, err
	}
	return &PredictiveZoneStrategy{db: db}, nil
}

func (s *PredictiveZoneStrategy) GetTargetZones(ctx context.Context) []Zone {
	// ML Clustering via SQL: 
	// We divide the map into a grid by rounding Lat/Lon to 2 decimal places (~1.1km accuracy).
	// We count how many pings happened in each grid over the last hour.
	// The top grids become our "Predicted Hotspots".
	
	query := `
		SELECT 
			ROUND(lat::numeric, 2) AS cluster_lat,
			ROUND(lon::numeric, 2) AS cluster_lon,
			COUNT(*) as ping_count
		FROM telemetry_history
		WHERE recorded_at >= NOW() - INTERVAL '1 hour'
		GROUP BY cluster_lat, cluster_lon
		ORDER BY ping_count DESC
		LIMIT 3; -- Pick the top 3 highest-density hotspots
	`

	rows, err := s.db.QueryxContext(ctx, query)
	if err != nil {
		slog.Error("Failed to run predictive clustering", "error", err)
		return []Zone{} // Fallback to empty if DB is busy
	}
	defer rows.Close()

	var zones []Zone
	hubIndex := 1

	for rows.Next() {
		var lat, lon float64
		var count int
		if err := rows.Scan(&lat, &lon, &count); err == nil {
			zones = append(zones, Zone{
				ID:             fmt.Sprintf("Predicted-Hotspot-%d", hubIndex),
				Lat:            lat,
				Lon:            lon,
				RadiusKm:       2.0, // Create a 2km radius catch-zone
				RequiredAssets: 5,   // Require 5 drones to pre-position here
				TargetClass:    uint16(domain.ClassDrone),
				TenantID:       "alpha_logistics",
			})
			hubIndex++
		}
	}

	if len(zones) > 0 {
		slog.Info("Predictive ML Engine updated hotspots", "clusters_found", len(zones))
	}
	return zones
}