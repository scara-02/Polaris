// This worker pulls data from Redis and saves it permanently to disk.

package stream

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/Akashpg-M/polaris/internal/core/domain"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	_ "github.com/lib/pq"
)

type PostgresArchiver struct {
	redisClient *redis.Client
	db          *sqlx.DB
}

func NewPostgresArchiver(redisURL, postgresURL string) (*PostgresArchiver, error) {
	rOpt, _ := redis.ParseURL(redisURL)
	redisClient := redis.NewClient(rOpt)

	db, err := sqlx.Connect("postgres", postgresURL)
	if err != nil {
		return nil, err
	}

	// Create a separate Consumer Group specifically for the archiver
	redisClient.XGroupCreateMkStream(context.Background(), StreamName, "polaris_archive_group", "$")

	return &PostgresArchiver{redisClient: redisClient, db: db}, nil
}

func (a *PostgresArchiver) Start(ctx context.Context) {
	slog.Info("Time-Series Archiver Worker Started")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Pull batches from the stream just like the spatial engine does
			streams, err := a.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    "polaris_archive_group",
				Consumer: "archiver-node-1",
				Streams:  []string{StreamName, ">"},
				Count:    500, // Process 500 pings at a time
				Block:    5 * time.Second,
			}).Result()

			if err != nil || len(streams) == 0 {
				continue
			}

			tx, err := a.db.Beginx() // Safe begin
			if err != nil {
					slog.Error("Failed to start DB transaction", "error", err)
					continue
			}

			var messageIDs []string
			var hasError bool

			for _, msg := range streams[0].Messages {
					var p domain.TelemetryPayload
					if err := json.Unmarshal([]byte(msg.Values["data"].(string)), &p); err != nil {
							continue // Skip bad JSON
					}

					// Safe Exec (No panic)
					_, err := tx.Exec(`INSERT INTO telemetry_history (tenant_id, node_id, asset_class, lat, lon, status, battery, recorded_at) 
							VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
							p.TenantID, p.NodeID, p.Class, p.Lat, p.Lon, p.Status, p.Battery, p.Timestamp)
					
					if err != nil {
							slog.Error("Failed to insert telemetry record", "node", p.NodeID, "error", err)
							hasError = true
							break // Stop processing this batch
					}
					
					messageIDs = append(messageIDs, msg.ID)
			}

			if hasError {
					tx.Rollback() // Abort the whole batch if something broke
					time.Sleep(1 * time.Second) // Backoff
					continue
			}

			// Commit the batch
			if err := tx.Commit(); err == nil && len(messageIDs) > 0 {
					a.redisClient.XAck(ctx, StreamName, "polaris_archive_group", messageIDs...)
					slog.Debug("Archived telemetry batch to PostgreSQL", "count", len(messageIDs))
			}
		}
	}
}