package domain

import "time"

// NodeStatus represents the universal FSM states for any IoT device
type NodeStatus string

const (
	StatusIdle        NodeStatus = "idle"         // Ready for orchestration
	StatusEnRoute     NodeStatus = "en_route"     // Actively moving to a target
	StatusActive      NodeStatus = "active"       // Performing its task (e.g., drone delivering)
	StatusMaintenance NodeStatus = "maintenance"  // Charging or broken
	StatusOffline     NodeStatus = "offline"      // Disconnected
)

// AssetClass uses Bitmasking to allow ultra-fast hardware-level filtering.
// By using uint16, we can support up to 16 distinct device categories natively.
type AssetClass uint16

const (
	ClassBike   AssetClass = 1 << 0 // 1  (Ground)
	ClassAuto   AssetClass = 1 << 1 // 2  (Ground)
	ClassSedan  AssetClass = 1 << 2 // 4  (Ground)
	ClassSUV    AssetClass = 1 << 3 // 8  (Ground)
	ClassDrone  AssetClass = 1 << 4 // 16 (Aerial - Bypasses street routing)
	ClassRobot  AssetClass = 1 << 5 // 32 (Ground - Warehouse/Sidewalk)
	ClassSensor AssetClass = 1 << 6 // 64 (Static - e.g., Smart Traffic Light)
)

// TelemetryPayload is the generic JSON structure expected from ANY device pinging the server.
type TelemetryPayload struct {
	TenantID  string                 `json:"tenant_id"` 
	NodeID    string                 `json:"node_id"`
	Class     AssetClass             `json:"asset_class"`
	Lat       float64                `json:"lat"`
	Lon       float64                `json:"lon"`
	Status    NodeStatus             `json:"status"`
	Battery   int                    `json:"battery,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"` // For custom device data
	Timestamp time.Time              `json:"timestamp"`
}