package ports

import "github.com/Akashpg-M/polaris/internal/core/entity"

type MatchingEngine interface {
	UpdateDriverLocation(update entity.LocationUpdate) error
	FindNearestDrivers(lat, lon float64, k int) ([]entity.Driver, error)
	BookDriver(driverID string) (int, error)
	ProgressTrip(tripID int, driverID string, currentStatus, newStatus entity.TripStatus) error
}