package repository

import (
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // Import driver anonymously

	"github.com/Akashpg-M/polaris/internal/core/entity"
)

type PostgresRepo struct {
	db *sqlx.DB
}

// NewPostgresRepo connects to the DB and runs migrations
func NewPostgresRepo(dsn string) (*PostgresRepo, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to db: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db ping failed: %w", err)
	}
	
	slog.Info("connected to postgres successfully")

	// AUTO-MIGRATE (Create tables if they don't exist)
	// In production, use a tool like 'migrate' or 'goose', but this works for v0.8
	schema := `
	CREATE TABLE IF NOT EXISTS drivers (
		id VARCHAR(50) PRIMARY KEY,
		status VARCHAR(20) NOT NULL,
		asset_type SMALLINT NOT NULL DEFAULT 4, -- Default to Sedan (4)
		lat DOUBLE PRECISION NOT NULL,
		lon DOUBLE PRECISION NOT NULL,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS trips (
		id SERIAL PRIMARY KEY,
		driver_id VARCHAR(50) REFERENCES drivers(id),
		rider_lat DOUBLE PRECISION NOT NULL,
		rider_lon DOUBLE PRECISION NOT NULL,
		status VARCHAR(30) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		started_at TIMESTAMP NULL,
		completed_at TIMESTAMP NULL
	);`
	
	_, err = db.Exec(schema)
	if err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return &PostgresRepo{db: db}, nil
}

// SaveDriver updates or inserts a driver
func (r *PostgresRepo) SaveDriver(id string, lat, lon float64, status string, asset uint8) error {
	query := `
		INSERT INTO drivers (id, lat, lon, status, asset_type, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (id) DO UPDATE 
		SET lat = $2, lon = $3, status = $4, asset_type = $5, updated_at = NOW();
	`
	_, err := r.db.Exec(query, id, lat, lon, status, asset)
	return err
}

// CreateTrip records a new booking and returns the generated Trip ID
func (r *PostgresRepo) CreateTrip(driverID string, rLat, rLon float64) (int, error) {
	// Notice the "RETURNING id" at the end, and we set initial status to 'driver_assigned'
	query := `
		INSERT INTO trips (driver_id, rider_lat, rider_lon, status) 
		VALUES ($1, $2, $3, 'driver_assigned') 
		RETURNING id
	`
	var tripID int
	// Use QueryRow to execute the insert and scan the returned ID
	err := r.db.QueryRow(query, driverID, rLat, rLon).Scan(&tripID)
	
	return tripID, err
}

// FetchActiveDrivers retrieves only drivers seen in the last 'n' seconds
func (r *PostgresRepo) FetchActiveDrivers(seconds int) ([]entity.Driver, error) {
	query := `
		SELECT id, lat, lon, status, asset_type as asset, updated_at 
		FROM drivers 
		WHERE status != 'offline' 
		AND updated_at > NOW() - make_interval(secs => $1)
	`
	var drivers []entity.Driver
	err := r.db.Select(&drivers, query, seconds)
	return drivers, err
}

// UpdateDriverHeartbeat updates location timestamp without heavy locking
// We use this for the "Write-Behind" strategy later.
func (r *PostgresRepo) UpdateDriverHeartbeat(id string, lat, lon float64) error {
    query := `
        INSERT INTO drivers (id, lat, lon, status, updated_at)
        VALUES ($1, $2, $3, 'available', NOW())
        ON CONFLICT (id) DO UPDATE 
        SET lat = $2, lon = $3, updated_at = NOW();
    `
    _, err := r.db.Exec(query, id, lat, lon)
    return err
}


// UpdateTripStatus uses Optimistic Locking to prevent race conditions.
// It explicitly requires the expectedCurrentStatus to ensure a driver
// isn't skipping steps (e.g., skipping 'Arrived' and going straight to 'Started').
func (r *PostgresRepo) UpdateTripStatus(tripID int, expectedCurrentStatus, newStatus entity.TripStatus) error {
	var query string
	
	// Dynamically set timestamps based on the new state
	if newStatus == entity.TripStarted {
		query = `UPDATE trips SET status = $1, started_at = NOW() WHERE id = $2 AND status = $3`
	} else if newStatus == entity.TripCompleted || newStatus == entity.TripCanceled {
		query = `UPDATE trips SET status = $1, completed_at = NOW() WHERE id = $2 AND status = $3`
	} else {
		query = `UPDATE trips SET status = $1 WHERE id = $2 AND status = $3`
	}

	result, err := r.db.Exec(query, newStatus, tripID, expectedCurrentStatus)
	if err != nil {
		return fmt.Errorf("db update failed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("invalid state transition or trip not found. Expected current status: %s", expectedCurrentStatus)
	}

	return nil
}