

package main

import (
    "encoding/json"
    "io"
    "log"
    "net/http"
    "os"
)

var gnnSidecarURL = getEnv("GNN_SIDECAR_URL", "http://gnn-sidecar:5050")

type HeatmapPayload struct {
    Layer       string             `json:"layer"`
    GeneratedAt int64              `json:"generated_at"`
    Data        map[string]float64 `json:"data"`
}

type RiderForecastPayload struct {
    GeneratedAt    int64       `json:"generated_at"`
    HorizonMinutes int         `json:"horizon_minutes"`
    Zones          []ZoneForecast `json:"zones"`
}

type ZoneForecast struct {
    ZoneID          string    `json:"zone_id"`
    Lat             float64   `json:"lat"`
    Lon             float64   `json:"lon"`
    PredictedDemand []float64 `json:"predicted_demand"`
    AvgDemand       float64   `json:"avg_demand"`
    HeatLevel       string    `json:"heat_level"`
}

// FetchPredictedHeatmap — call this every 30s in your heatmap refresh goroutine
func FetchPredictedHeatmap() (*HeatmapPayload, error) {
    resp, err := http.Get(gnnSidecarURL + "/demand/heatmap")
    if err != nil {
        log.Printf("[GNN] heatmap fetch failed: %v", err)
        return nil, err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    var payload HeatmapPayload
    json.Unmarshal(body, &payload)
    return &payload, nil
}

// FetchRiderForecast — call this when a rider client requests demand insight
func FetchRiderForecast() (*RiderForecastPayload, error) {
    resp, err := http.Get(gnnSidecarURL + "/demand/forecast")
    if err != nil {
        log.Printf("[GNN] rider forecast fetch failed: %v", err)
        return nil, err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    var payload RiderForecastPayload
    json.Unmarshal(body, &payload)
    return &payload, nil
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" { return v }
    return fallback
}