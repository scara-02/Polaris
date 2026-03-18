package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Metrics counters (using atomic for thread-safe high-speed counting)
var (
	activeConnections int64
	messagesSent      int64
	connectionErrors  int64
)

// Payload matches the Polaris domain model
type Payload struct {
	TenantID string  `json:"tenant_id"`
	NodeID   string  `json:"node_id"`
	Class    uint16  `json:"asset_class"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Status   string  `json:"status"`
	Battery  int     `json:"battery"`
}

func main() {
	// Configurable flags so you can scale up gradually
	targetNodes := flag.Int("nodes", 10000, "Number of concurrent drones to simulate")
	serverURL := flag.String("url", "ws://localhost:6080/ws/telemetry", "Gateway WebSocket URL")
	flag.Parse()

	log.Printf("🚀 Initiating Polaris Stress Test...")
	log.Printf("Targeting %d concurrent drones on %s", *targetNodes, *serverURL)

	var wg sync.WaitGroup

	// Start the live metrics dashboard
	go printMetricsDashboard()

	// To prevent crashing the local network stack by opening 10,000 TCP ports 
	// in a single millisecond, we stagger the connections using a ticker.
	connectionRate := time.Tick(2 * time.Millisecond) // 500 new connections per second

	for i := 1; i <= *targetNodes; i++ {
		<-connectionRate
		wg.Add(1)
		go simulateDrone(i, *serverURL, &wg)
	}

	log.Println("All drones deployed. Waiting for simulation to finish (Press Ctrl+C to abort)...")
	wg.Wait()
}

func simulateDrone(id int, wsURL string, wg *sync.WaitGroup) {
	defer wg.Done()

	// 1. Establish the Uplink
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		atomic.AddInt64(&connectionErrors, 1)
		return
	}
	defer conn.Close()

	atomic.AddInt64(&activeConnections, 1)
	defer atomic.AddInt64(&activeConnections, -1)

	nodeID := fmt.Sprintf("STRESS-DRONE-%d", id)

	// Spawn in a random location near Chennai
	lat := 13.04 + (rand.Float64() * 0.1)
	lon := 80.24 + (rand.Float64() * 0.1)

	// 2. The Telemetry Loop
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Simulate slight movement
		lat += (rand.Float64() - 0.5) * 0.001
		lon += (rand.Float64() - 0.5) * 0.001

		payload := Payload{
			TenantID: "alpha_logistics",
			NodeID:   nodeID,
			Class:    16, // Drone
			Lat:      lat,
			Lon:      lon,
			Status:   "en_route",
			Battery:  rand.Intn(100),
		}

		msg, _ := json.Marshal(payload)
		err := conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			atomic.AddInt64(&connectionErrors, 1)
			break // Connection died, exit the loop
		}
		
		atomic.AddInt64(&messagesSent, 1)
	}
}

// printMetricsDashboard clears the console line and prints live stats
func printMetricsDashboard() {
	ticker := time.NewTicker(1 * time.Second)
	var lastMessagesSent int64

	for range ticker.C {
		currentMessages := atomic.LoadInt64(&messagesSent)
		throughput := currentMessages - lastMessagesSent
		lastMessagesSent = currentMessages

		fmt.Printf("\r📡 ACTIVE UPLINKS: %d | ⚡ THROUGHPUT: %d msgs/sec | ❌ ERRORS: %d | 📦 TOTAL SENT: %d",
			atomic.LoadInt64(&activeConnections),
			throughput,
			atomic.LoadInt64(&connectionErrors),
			currentMessages,
		)
	}
}