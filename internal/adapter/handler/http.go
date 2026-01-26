package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/Akashpg-M/polaris/internal/core/entity"
	"github.com/Akashpg-M/polaris/internal/core/ports"
)

type HTTPHandler struct {
	engine ports.MatchingEngine
}

func NewHTTPHandler(engine ports.MatchingEngine) *HTTPHandler {
	return &HTTPHandler{engine: engine}
}

// 1. POST /driver/location
func (h *HTTPHandler) UpdateLocation(c *gin.Context) {
	var update entity.LocationUpdate

	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
		return
	}

	if err := h.engine.UpdateDriverLocation(update); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update location"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

// 2. GET /ride/match?lat=10.0&lon=20.0
func (h *HTTPHandler) FindMatches(c *gin.Context) {
	latStr := c.Query("lat")
	lonStr := c.Query("lon")

	lat, err1 := strconv.ParseFloat(latStr, 64)
	lon, err2 := strconv.ParseFloat(lonStr, 64)

	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid lat/lon parameters"})
		return
	}

	matches, err := h.engine.FindNearestDrivers(lat, lon, 5)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Search failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count":   len(matches),
		"matches": matches,
	})
}

func (h *HTTPHandler) BookRide(c *gin.Context) {
	var req struct {
		DriverID string `json:"driver_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Request"})
		return
	}

	// Call Engine
	if err := h.engine.BookDriver(req.DriverID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "booked", "driver_id": req.DriverID})
}