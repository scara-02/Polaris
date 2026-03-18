package orchestrator

import (
	"context"
	"github.com/Akashpg-M/polaris/internal/core/domain"
)

// StaticZoneStrategy simulates a database table of logistics hubs or smart-city sectors
type StaticZoneStrategy struct {}

func (s *StaticZoneStrategy) GetTargetZones(ctx context.Context) []Zone {
	return []Zone{
		{
			ID:             "Hub-Guindy",
			Lat:            13.0067,
			Lon:            80.2206,
			RadiusKm:       5.0,
			RequiredAssets: 3,
			TargetClass:    uint16(domain.ClassDrone),
            TenantID:       "alpha_logistics",
		},
		{
			ID:             "Hub-Adyar",
			Lat:            13.0012,
			Lon:            80.2565,
			RadiusKm:       3.0,
			RequiredAssets: 2,
			TargetClass:    uint16(domain.ClassDrone),
            TenantID:       "alpha_logistics",
		},
	}
}