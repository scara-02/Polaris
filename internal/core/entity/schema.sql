-- permanent driver info
CREATE TABLE IF NOT EXISTS drivers (
    id VARCHAR(50) PRIMARY KEY,
    status VARCHAR(20) NOT NULL, -- 'available', 'booked', 'offline'
    lat DOUBLE PRECISION NOT NULL,
    lon DOUBLE PRECISION NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- booking history
CREATE TABLE IF NOT EXISTS trips (
    id SERIAL PRIMARY KEY,
    driver_id VARCHAR(50) REFERENCES drivers(id),
    rider_lat DOUBLE PRECISION NOT NULL,
    rider_lon DOUBLE PRECISION NOT NULL,
    status VARCHAR(20) NOT NULL, -- 'started', 'completed'
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);