package entity

import "time"

type DriverStatus string

const (
	DriverAvailable DriverStatus = "available"
	DriverBooked    DriverStatus = "booked"
	DriverOffline   DriverStatus = "offline"
)

type AssetType uint8

const (
	AssetBike  AssetType = 1 << 0 // 1  (0001)
	AssetAuto  AssetType = 1 << 1 // 2  (0010)
	AssetSedan AssetType = 1 << 2 // 4  (0100)
	AssetSUV   AssetType = 1 << 3 // 8  (1000)
	AssetAll   AssetType = 255    // Matches everything
)

type Driver struct {
	ID        string       `json:"id" db:"id"`
	Lat       float64      `json:"lat" db:"lat"`
	Lon       float64      `json:"lon" db:"lon"`
	Status    DriverStatus `json:"status" db:"status"`
	Asset         AssetType    `json:"asset_type"`
	UpdatedAt time.Time    `json:"updated_at" db:"updated_at"`
	ETA           float64  `json:"eta_seconds,omitempty"`
  RouteDistance float64  `json:"route_distance_meters,omitempty"`
}

type LocationUpdate struct {
	DriverID string  `json:"driver_id"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Asset    AssetType `json:"asset_type"`
}