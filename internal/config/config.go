package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv    string
	Port      string
	LogLevel  string
	MapWidth  float64
	MapHeight float64
	DBUrl     string
	RedisUrl  string
}

func Load() *Config {
	// Try loading .env file (It's okay if it fails in production)
	if err := godotenv.Load(); err != nil {
		// Only log if we are locally developing, otherwise silence
		if os.Getenv("APP_ENV") != "production" {
			log.Println("No .env file found, using system environment variables")
		}
	}

	return &Config{
		AppEnv:    getEnv("APP_ENV", "development"),
		Port:      getEnv("PORT", "8080"),
		LogLevel:  getEnv("LOG_LEVEL", "info"),
		MapWidth:  getEnvFloat("MAP_WIDTH", 1000.0),
		MapHeight: getEnvFloat("MAP_HEIGHT", 1000.0),
		DBUrl:     getDBUrl(),
		RedisUrl:  getRedisUrl(),
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

func getDBUrl() string {
	// Format: postgres://user:password@host:port/dbname
	return "postgres://" + getEnv("DB_USER", "user") + ":" +
		getEnv("DB_PASSWORD", "pass") + "@" +
		getEnv("DB_HOST", "localhost") + ":" +
		getEnv("DB_PORT", "5433") + "/" +
		getEnv("DB_NAME", "polaris") +
		"?sslmode=disable"   // enabled due to some docker desktop issue,
}

func getRedisUrl() string {
	return getEnv("REDIS_HOST", "localhost") + ":" + 
				 getEnv("REDIS_PORT", "6379")
}
