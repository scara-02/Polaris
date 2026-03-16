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

			// We use a SQL Transaction for a high-speed Bulk Insert
			tx := a.db.MustBegin()
			var messageIDs []string

			for _, msg := range streams[0].Messages {
				var p domain.TelemetryPayload
				json.Unmarshal([]byte(msg.Values["data"].(string)), &p)

				tx.MustExec(`INSERT INTO telemetry_history (tenant_id, node_id, asset_class, lat, lon, status, battery, recorded_at) 
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
					p.TenantID, p.NodeID, p.Class, p.Lat, p.Lon, p.Status, p.Battery, p.Timestamp)
				
				messageIDs = append(messageIDs, msg.ID)
			}

			// Commit the batch to the hard drive
			err = tx.Commit()
			if err == nil {
				// Tell Redis we successfully saved them
				a.redisClient.XAck(ctx, StreamName, "polaris_archive_group", messageIDs...)
				slog.Debug("Archived telemetry batch to PostgreSQL", "count", len(messageIDs))
			}
		}
	}
}