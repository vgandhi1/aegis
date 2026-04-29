package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists work orders in PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// New returns a repository backed by the given pool.
func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// InsertWorkOrder records a new production session and returns its id.
func (r *Repository) InsertWorkOrder(ctx context.Context, stationID, vin, firmware string) (int64, error) {
	const q = `
		INSERT INTO work_orders (station_id, vin, firmware_version, status)
		VALUES ($1, $2, $3, 'active')
		RETURNING id`
	var id int64
	err := r.pool.QueryRow(ctx, q, stationID, vin, firmware).Scan(&id)
	return id, err
}

// WorkOrder is a row exposed to the dashboard API.
type WorkOrder struct {
	ID              int64     `json:"id"`
	StationID       string    `json:"station_id"`
	VIN             string    `json:"vin"`
	FirmwareVersion string    `json:"firmware_version"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

// ListRecent returns the most recent work orders.
func (r *Repository) ListRecent(ctx context.Context, limit int) ([]WorkOrder, error) {
	const q = `
		SELECT id, station_id, vin, firmware_version, status, created_at
		FROM work_orders
		ORDER BY created_at DESC
		LIMIT $1`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkOrder
	for rows.Next() {
		var w WorkOrder
		if err := rows.Scan(&w.ID, &w.StationID, &w.VIN, &w.FirmwareVersion, &w.Status, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// CountWorkOrders returns total rows for status reporting.
func (r *Repository) CountWorkOrders(ctx context.Context) (int64, error) {
	var n int64
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM work_orders`).Scan(&n)
	return n, err
}
