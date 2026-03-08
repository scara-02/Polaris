package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRepo struct {
	client *redis.Client
}

func NewRedisRepo(addr string) (*RedisRepo, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisRepo{client: client}, nil
}

// UpdateLocation writes the driver's position to a Redis Geo Set
// This is incredibly fast (microsecond latency).
func (r *RedisRepo) UpdateLocation(ctx context.Context, driverID string, lat, lon float64) error {
	// GEOADD key lon lat member
	return r.client.GeoAdd(ctx, "drivers:locations", &redis.GeoLocation{
		Name:      driverID,
		Longitude: lon,
		Latitude:  lat,
	}).Err()
}

// AcquireLock tries to get a distributed lock for a specific driver
// Returns true if we got the lock, false if someone else has it.
func (r *RedisRepo) AcquireLock(ctx context.Context, driverID string) (bool, error) {
	key := fmt.Sprintf("lock:driver:%s", driverID)
	// SET key value NX EX 10 (Set if Not Exists, Expire in 10s)
	success, err := r.client.SetNX(ctx, key, "locked", 10*time.Second).Result()
	return success, err
}

// ReleaseLock removes the lock (call this after booking is done or failed)
func (r *RedisRepo) ReleaseLock(ctx context.Context, driverID string) error {
	key := fmt.Sprintf("lock:driver:%s", driverID)
	return r.client.Del(ctx, key).Err()
}