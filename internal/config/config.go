package config

import (
	"os"
	"strconv"
)

type Config struct {
	MapWidth  float64
	MapHeight float64
	LogLevel  string
}

func Load() *Config {
	return &Config{
		MapWidth:  getEnvFloat("MAP_WIDTH", 1000.0),
		MapHeight: getEnvFloat("MAP_HEIGHT", 1000.0),
		LogLevel:  getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	valStr := getEnv(key, "")
	if valStr == "" {
		return fallback
	}
	if val, err := strconv.ParseFloat(valStr, 64); err == nil { // convert string to float
		return val
	}
	return fallback
}