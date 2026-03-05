package osrm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"github.com/Akashpg-M/polaris/internal/core/entity"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(url string) *Client {
	return &Client{
		baseURL: url,
		httpClient: &http.Client{
			Timeout: 2 * time.Second, // Strict timeout so we don't block the API
		},
	}
}

type tableResponse struct {
	Code      string      `json:"code"`
	Durations [][]float64 `json:"durations"`
	Distances [][]float64 `json:"distances"`
}

// CalculateETAs takes a rider location and a list of candidate drivers, returning them updated with actual ETAs.
func (c *Client) CalculateETAs(riderLat, riderLon float64, candidates []entity.Driver) ([]entity.Driver, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}

	// OSRM Format: {lon},{lat};{lon},{lat}...
	// Source (0) is Rider. Destinations (1...N) are Drivers.
	var coords []string
	coords = append(coords, fmt.Sprintf("%f,%f", riderLon, riderLat)) // Source

	for _, d := range candidates {
		coords = append(coords, fmt.Sprintf("%f,%f", d.Lon, d.Lat)) // Destinations
	}

	coordString := strings.Join(coords, ";")
	
	// sources=0 means distance FROM rider TO all others. 
	// annotations=duration,distance gets both time and meters.
	reqURL := fmt.Sprintf("%s/table/v1/driving/%s?sources=0&annotations=duration,distance", c.baseURL, coordString)

	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("osrm request failed: %w", err)
	}
	defer resp.Body.Close()

	var result tableResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode osrm response: %w", err)
	}

	if result.Code != "Ok" || len(result.Durations) == 0 {
		return nil, fmt.Errorf("osrm returned non-ok code: %s", result.Code)
	}

	// Map the results back to the drivers
	for i := range candidates {
		// result.Durations[0] is the array of times from the Rider (source 0) to all targets
		// The targets are offset by +1 because target 0 is the rider to themselves.
		candidates[i].ETA = result.Durations[0][i+1]
		candidates[i].RouteDistance = result.Distances[0][i+1]
	}

	return candidates, nil
}