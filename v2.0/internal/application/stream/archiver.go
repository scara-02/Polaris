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

			tx, err := a.db.Beginx() 
      if err != nil {
          slog.Error("Failed to start DB transaction", "error", err)
          continue
      }

      var successfulMessageIDs []string
      var transactionAborted bool

      for _, msg := range streams[0].Messages {
          var p domain.TelemetryPayload
          
          // 1. Unmarshal check (Safe to continue, DB is untouched)
          if err := json.Unmarshal([]byte(msg.Values["data"].(string)), &p); err != nil {
              slog.Warn("Dropping malformed JSON payload", "msg_id", msg.ID)
              a.redisClient.XAck(ctx, StreamName, "polaris_archive_group", msg.ID)
              continue 
          }

          // 2. Insert check
          _, err := tx.Exec(`INSERT INTO telemetry_history (tenant_id, node_id, asset_class, lat, lon, status, battery, recorded_at) 
              VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
              p.TenantID, p.NodeID, p.Class, p.Lat, p.Lon, p.Status, p.Battery, p.Timestamp)
          
          if err != nil {
              slog.Error("Dropping bad telemetry record (DB Constraint Failed)", "node", p.NodeID, "error", err)
              a.redisClient.XAck(ctx, StreamName, "polaris_archive_group", msg.ID) // Drop the bad pill
              
              // CRITICAL: The transaction is dead. Rollback and abort the rest of this loop.
              tx.Rollback()
              transactionAborted = true
              break 
          }
          
          // 3. Success
          successfulMessageIDs = append(successfulMessageIDs, msg.ID)
      }

      // If the transaction died, skip the commit phase entirely. 
      // Redis will resend the un-acked good messages in the next batch.
      if transactionAborted {
          continue 
      }

      // Commit the good ones
      if len(successfulMessageIDs) > 0 {
          if err := tx.Commit(); err == nil {
              a.redisClient.XAck(ctx, StreamName, "polaris_archive_group", successfulMessageIDs...)
              slog.Debug("Archived telemetry batch to PostgreSQL", "count", len(successfulMessageIDs))
          } else {
              slog.Error("Failed to commit batch to PostgreSQL", "error", err)
              tx.Rollback()
          }
      } else {
          tx.Rollback() // Nothing to save (e.g., if all messages were bad JSON)
      }
		}
	}
}