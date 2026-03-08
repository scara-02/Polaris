package pricing

import (
	"github.com/Akashpg-M/polaris/internal/core/entity"
)

// CalculateFare computes the price in INR (₹) based on asset type, real distance, time, and surge.
func CalculateFare(asset entity.AssetType, distanceMeters, durationSeconds float64, surge float64) float64 {
	km := distanceMeters / 1000.0
	mins := durationSeconds / 60.0

	var base, perKm, perMin float64

	switch asset {
	case entity.AssetBike:
		base, perKm, perMin = 20.0, 5.0, 1.0
	case entity.AssetAuto:
		base, perKm, perMin = 30.0, 10.0, 1.5
	case entity.AssetSedan:
		base, perKm, perMin = 50.0, 15.0, 2.0
	case entity.AssetSUV:
		base, perKm, perMin = 80.0, 20.0, 3.0
	default:
		base, perKm, perMin = 50.0, 15.0, 2.0 // Fallback to Sedan
	}

	total := base + (km * perKm) + (mins * perMin)
	
	return total * surge
}