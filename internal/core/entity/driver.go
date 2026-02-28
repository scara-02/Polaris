package entity

import "time"

type DriverStatus string

const (
	DriverAvailable DriverStatus = "available"
	DriverBooked    DriverStatus = "booked"
	DriverOffline   DriverStatus = "offline"
)

type Driver struct {
	ID        string       `json:"id" db:"id"`
	Lat       float64      `json:"lat" db:"lat"`
	Lon       float64      `json:"lon" db:"lon"`
	Status    DriverStatus `json:"status" db:"status"`
	UpdatedAt time.Time    `json:"updated_at" db:"updated_at"`
}

type LocationUpdate struct {
	DriverID string  `json:"driver_id"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
}