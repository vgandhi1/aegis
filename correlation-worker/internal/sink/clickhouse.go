package sink

import (
	"context"
	"log"
	"time"

	"aegis/correlation-worker/internal/models"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// EnrichedWriter batches enriched telemetry into ClickHouse.
type EnrichedWriter struct {
	conn driver.Conn
}

// NewEnrichedWriter opens a native ClickHouse connection.
func NewEnrichedWriter(ctx context.Context, addrs []string) (*EnrichedWriter, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: addrs,
		Auth: clickhouse.Auth{Database: "aegis"},
	})
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &EnrichedWriter{conn: conn}, nil
}

// Close releases the connection.
func (w *EnrichedWriter) Close() error {
	return w.conn.Close()
}

// Run drains out until ctx is done, batching inserts.
func (w *EnrichedWriter) Run(ctx context.Context, out <-chan models.EnrichedMessage) {
	const batchMax = 500
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()

	buf := make([]models.EnrichedMessage, 0, batchMax)
	flush := func() {
		if len(buf) == 0 {
			return
		}
		if err := w.insertBatch(ctx, buf); err != nil {
			log.Printf("clickhouse insert: %v", err)
		}
		buf = buf[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case m, ok := <-out:
			if !ok {
				flush()
				return
			}
			buf = append(buf, m)
			if len(buf) >= batchMax {
				flush()
			}
		case <-tick.C:
			flush()
		}
	}
}

func (w *EnrichedWriter) insertBatch(ctx context.Context, rows []models.EnrichedMessage) error {
	batch, err := w.conn.PrepareBatch(ctx, `
		INSERT INTO aegis.enriched_telemetry (station_id, vin, firmware, torque, ts)
	`)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := batch.Append(r.StationID, r.VIN, r.Firmware, r.Torque, r.Timestamp); err != nil {
			return err
		}
	}
	return batch.Send()
}
