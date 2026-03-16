package handler

import (
	"net/http"
	"strconv"

	"github.com/Akashpg-M/polaris/internal/application/spatial"
	"github.com/gin-gonic/gin"
)

type MatchHandler struct {
	engine *spatial.Engine
}

func NewMatchHandler(engine *spatial.Engine) *MatchHandler {
	return &MatchHandler{engine: engine}
}

// GetNearestNodes handles GET /api/v1/nodes/match
func (h *MatchHandler) GetNearestNodes(c *gin.Context) {
	tenantID := c.Query("tenant_id") 
	if tenantID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing tenant identity"})
			return
	}
	// Parse required query parameters
	lat, errLat := strconv.ParseFloat(c.Query("lat"), 64)
	lon, errLon := strconv.ParseFloat(c.Query("lon"), 64)
	radius, errRad := strconv.ParseFloat(c.DefaultQuery("radius_km", "10.0"), 64)
	assetClass, errClass := strconv.ParseUint(c.DefaultQuery("class", "16"), 10, 16) // Default 16 = Drone

	
	if errLat != nil || errLon != nil || errRad != nil || errClass != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters. lat and lon are required."})
		return
	}

	// Query the In-Memory Engine
	matches := h.engine.FindNearest(tenantID, lat, lon, radius, uint16(assetClass))
	// Return standard JSON response
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"count":  len(matches),
		"data":   matches,
	})
}