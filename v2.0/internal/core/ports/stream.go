package ports

import (
	"context"
	"github.com/Akashpg-M/polaris/internal/core/domain"
)

// TelemetryPublisher defines how incoming IoT pings are buffered before processing.
type TelemetryPublisher interface {
	// Publish drops the raw telemetry into a stream/queue immediately.
	Publish(ctx context.Context, payload domain.TelemetryPayload) error
}