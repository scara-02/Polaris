package stream

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Akashpg-M/polaris/internal/application/spatial"
	"github.com/Akashpg-M/polaris/internal/core/domain"
	"github.com/redis/go-redis/v9"
)

const (
	StreamName    = "telemetry:ingress"
	ConsumerGroup = "polaris_engine_group"
)

type RedisConsumer struct {
	client *redis.Client
	engine *spatial.Engine
}

func NewRedisConsumer(redisURL string, engine *spatial.Engine) (*RedisConsumer, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	// Attempt to create the Consumer Group. Ignore error if it already exists.
	err = client.XGroupCreateMkStream(context.Background(), StreamName, ConsumerGroup, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		slog.Error("Failed to create consumer group", "err", err)
	}

	return &RedisConsumer{
		client: client,
		engine: engine,
	}, nil
}

// Start begins pulling telemetry batches from Redis
func (c *RedisConsumer) Start(ctx context.Context, workerID string) {
	slog.Info("Starting Redis Consumer Worker", "worker_id", workerID)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Shutting down consumer worker", "worker_id", workerID)
			return
		default:
			// Pull up to 1,000 messages at a time. Block for 2 seconds if stream is empty.
			streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    ConsumerGroup,
				Consumer: workerID,
				Streams:  []string{StreamName, ">"},
				Count:    1000,
				Block:    2 * time.Second,
			}).Result()

			if err != nil {
				if err != redis.Nil { // redis.Nil just means timeout (empty stream), which is fine
					slog.Error("Failed to read from stream", "err", err)
					time.Sleep(1 * time.Second)
				}
				continue
			}

			var batch []domain.TelemetryPayload
			var messageIDs []string

			// Parse the incoming JSON batch
			for _, message := range streams[0].Messages {
				var payload domain.TelemetryPayload
				dataStr := message.Values["data"].(string)

				if err := json.Unmarshal([]byte(dataStr), &payload); err == nil {
					batch = append(batch, payload)
					messageIDs = append(messageIDs, message.ID)
				}
			}

			// Process the batch in the Spatial Engine
			if len(batch) > 0 {
				c.engine.BatchUpdate(batch)

				// Acknowledge the messages so Redis drops them from the pending list
				c.client.XAck(ctx, StreamName, ConsumerGroup, messageIDs...)
			}
		}
	}
}