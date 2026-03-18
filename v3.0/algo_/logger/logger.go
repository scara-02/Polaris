package logger

import (
	"log/slog"
	"os"
)

// Init configures a global, structured JSON logger for enterprise observability.
func Init() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug, // Set to Info in production
	}
	
	// Create a JSON handler that outputs to standard out
	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	
	// Set this as the default logger for the entire Go application
	slog.SetDefault(logger)
	slog.Info("Structured JSON logging initialized")
}