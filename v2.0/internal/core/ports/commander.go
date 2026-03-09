package ports

// FleetCommander defines how the central brain pushes directives to physical nodes
type FleetCommander interface {
	SendCommand(nodeID string, payload interface{}) error
}