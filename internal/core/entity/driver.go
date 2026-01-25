package entity

import "time"

type DriverStatus string

const (
	DriverAvailable DriverStatus = "available"
	DriverBooked    DriverStatus = "booked"
	DriverOffline   DriverStatus = "offline"
)

type Driver struct {
	ID        string
	Lat       float64
	Lon       float64
	Status    DriverStatus
	UpdatedAt time.Time
}

type LocationUpdate struct {
	DriverID string  `json:"driver_id"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
}