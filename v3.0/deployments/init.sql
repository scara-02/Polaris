CREATE TABLE IF NOT EXISTS telemetry_history (
    id SERIAL PRIMARY KEY,
    tenant_id VARCHAR(50) NOT NULL,
    node_id VARCHAR(50) NOT NULL,
    asset_class SMALLINT NOT NULL,
    lat DOUBLE PRECISION NOT NULL,
    lon DOUBLE PRECISION NOT NULL,
    status VARCHAR(20) NOT NULL,
    battery INT,
    recorded_at TIMESTAMP NOT NULL
);

-- Index for fast historical reporting (e.g., "Where was Drone-1001 yesterday?")
CREATE INDEX idx_telemetry_node_time ON telemetry_history(node_id, recorded_at DESC);