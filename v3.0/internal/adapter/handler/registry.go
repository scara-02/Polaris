package handler

import (
	"fmt"
	"sync"
	"github.com/gorilla/websocket"
)

// Client wraps a websocket connection with a mutex to prevent concurrent write panics
type Client struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

// ConnectionRegistry holds all live IoT uplinks
type ConnectionRegistry struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

func NewConnectionRegistry() *ConnectionRegistry {
	return &ConnectionRegistry{
		clients: make(map[string]*Client),
	}
}

func (r *ConnectionRegistry) Register(nodeID string, conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[nodeID] = &Client{conn: conn}
}

func (r *ConnectionRegistry) Unregister(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.clients, nodeID)
}

// SendCommand implements ports.FleetCommander
func (r *ConnectionRegistry) SendCommand(nodeID string, payload interface{}) error {
	r.mu.RLock()
	client, exists := r.clients[nodeID]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("node %s is unreachable", nodeID)
	}

	// Lock the specific client for thread-safe writing
	client.mu.Lock()
	defer client.mu.Unlock()
	
	return client.conn.WriteJSON(payload)
}

// GetActiveCount returns the number of currently connected IoT devices
func (r *ConnectionRegistry) GetActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}