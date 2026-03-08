package entity

import "time"

// TripStatus defines the strict allowed states for a ride
type TripStatus string

const (
	TripRequested      TripStatus = "requested"
	TripDriverAssigned TripStatus = "driver_assigned"
	TripDriverArrived  TripStatus = "driver_arrived"
	TripStarted        TripStatus = "trip_started"
	TripCompleted      TripStatus = "completed"
	TripCanceled       TripStatus = "canceled"
)

type Trip struct {
	ID          int        `json:"id"`
	DriverID    string     `json:"driver_id"`
	RiderLat    float64    `json:"rider_lat"`
	RiderLon    float64    `json:"rider_lon"`
	Status      TripStatus `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ValidTransition checks if moving from one state to another is legally allowed
func ValidTransition(current, next TripStatus) bool {
	transitions := map[TripStatus][]TripStatus{
		TripRequested:      {TripDriverAssigned, TripCanceled},
		TripDriverAssigned: {TripDriverArrived, TripCanceled},
		TripDriverArrived:  {TripStarted, TripCanceled},
		TripStarted:        {TripCompleted},
		TripCompleted:      {}, // Terminal state
		TripCanceled:       {}, // Terminal state
	}

	allowed, exists := transitions[current]
	if !exists {
		return false
	}
	for _, status := range allowed {
		if status == next {
			return true
		}
	}
	return false
}