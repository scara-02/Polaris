package handler

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/Akashpg-M/polaris/internal/core/domain"
	"github.com/Akashpg-M/polaris/internal/core/ports"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, restrict this to specific domains/JWTs
	},
}

type IngestionHandler struct {
	publisher ports.TelemetryPublisher
	registry  *ConnectionRegistry
}

func NewIngestionHandler(pub ports.TelemetryPublisher, reg *ConnectionRegistry) *IngestionHandler {
	return &IngestionHandler{publisher: pub, registry: reg}
}

// HandleIoTConnection upgrades the HTTP request to a persistent WebSocket
func (h *IngestionHandler) HandleIoTConnection(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade websocket: %v", err)
		return
	}
	defer conn.Close()

	log.Println("[Gateway] New IoT Node Connected")

	var nodeID string

	for {
		var payload domain.TelemetryPayload		
		err := conn.ReadJSON(&payload)
		if err != nil {
			log.Printf("[Gateway] Node disconnected or invalid payload: %v", err)
			break
		}

		if nodeID == "" {
			nodeID = payload.NodeID
			h.registry.Register(nodeID, conn)
			slog.Debug("IoT Uplink Established", "node", nodeID)
		}
		// Inject server-side timestamp for accuracy
		payload.Timestamp = time.Now().UTC()

		// 2. Push immediately to Redis Stream (The Shock Absorber)
		// We use a short timeout context to prevent a dead Redis instance from locking the goroutine
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = h.publisher.Publish(ctx, payload)
		cancel()

		if err != nil {
			log.Printf("[Gateway Error] Failed to buffer telemetry: %v", err)
			// Optional: Notify the device that the ping failed
			_ = conn.WriteJSON(map[string]string{"error": "buffer_overflow"})
			continue
		}

		// 3. Acknowledge receipt (Keep-Alive)
		_ = conn.WriteJSON(map[string]string{"status": "buffered"})
	}

	if nodeID != "" {
		h.registry.Unregister(nodeID)
		slog.Debug("IoT Uplink Lost", "node", nodeID)
	}
}