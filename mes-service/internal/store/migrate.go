package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
CREATE TABLE IF NOT EXISTS work_orders (
	id SERIAL PRIMARY KEY,
	station_id VARCHAR(32) NOT NULL,
	vin VARCHAR(17) NOT NULL,
	firmware_version VARCHAR(64) NOT NULL,
	status VARCHAR(32) NOT NULL DEFAULT 'active',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_work_orders_station ON work_orders (station_id);
`

// Migrate applies the relational schema for the mock MES.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, schema)
	return err
}
