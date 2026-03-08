package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Akashpg-M/polaris/internal/core/domain"
	"github.com/redis/go-redis/v9"
)

const TelemetryStreamKey = "telemetry:ingress"

type RedisStreamAdapter struct {
	client *redis.Client
}

func NewRedisStreamAdapter(redisURL string) (*RedisStreamAdapter, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid redis url: %w", err)
	}
	
	client := redis.NewClient(opts)
	
	// Ping to ensure connection is alive
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisStreamAdapter{client: client}, nil
}

// Publish implements ports.TelemetryPublisher using Redis XADD
func (r *RedisStreamAdapter) Publish(ctx context.Context, payload domain.TelemetryPayload) error {
	// Serialize the payload to standard JSON
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize telemetry: %w", err)
	}

	// XADD pushes the event to the Redis Stream. 
	// MaxLen limits the stream size to 100,000 to prevent OOM (Out Of Memory) crashes if consumers die.
	err = r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: TelemetryStreamKey,
		MaxLen: 100000, 
		Approx: true,
		Values: map[string]interface{}{
			"data": string(data),
		},
	}).Err()

	if err != nil {
		return fmt.Errorf("redis stream xadd failed: %w", err)
	}

	return nil
}