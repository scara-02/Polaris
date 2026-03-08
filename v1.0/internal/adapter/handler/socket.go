package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/Akashpg-M/polaris/internal/core/entity"
)

// Promotes HTTP -> WS
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CORS for WebSockets (for localhost dev)
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Route: GET /ws/driver
func (h *HTTPHandler) DriverSocket(c *gin.Context) {
	// connection upgrade
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("failed to upgrade websocket", "error", err)
		return
	}
	defer conn.Close()

	slog.Info("new driver connected", "remote_addr", conn.RemoteAddr())

	for {
		var update entity.LocationUpdate
		
		err := conn.ReadJSON(&update)
		if err != nil {
			slog.Warn("websocket disconnected", "error", err)
			break 
		}

		if err := h.engine.UpdateDriverLocation(update); err != nil {
			slog.Error("engine update failed", "error", err)
		}
	}
}