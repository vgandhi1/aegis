CREATE DATABASE IF NOT EXISTS aegis;

CREATE TABLE IF NOT EXISTS aegis.enriched_telemetry
(
    station_id String,
    vin        String,
    firmware   String,
    torque     Float64,
    ts         Int64,
    ingested_at DateTime DEFAULT now()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(toDateTime(intDiv(ts, 1000)))
ORDER BY (station_id, ts)
TTL toDateTime(intDiv(ts, 1000)) + INTERVAL 90 DAY;
