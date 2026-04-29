package processor

import (
	"context"

	"aegis/correlation-worker/internal/models"
)

// StateReader fetches enterprise context for a station.
type StateReader interface {
	GetCurrentState(stationID string) (vin, firmware string)
}

// StreamEnricher joins high-frequency PLC data with cached MES state.
type StreamEnricher struct {
	stateCache StateReader
}

// NewStreamEnricher constructs an enricher with the given state source.
func NewStreamEnricher(cache StateReader) *StreamEnricher {
	return &StreamEnricher{stateCache: cache}
}

// Run processes the incoming telemetry stream until ctx is canceled or in is closed.
func (e *StreamEnricher) Run(ctx context.Context, in <-chan models.PLCMessage, out chan<- models.EnrichedMessage) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-in:
			if !ok {
				return
			}
			vin, fw := e.stateCache.GetCurrentState(msg.StationID)
			if vin == "" {
				continue
			}
			out <- models.EnrichedMessage{
				PLCMessage: msg,
				VIN:        vin,
				Firmware:   fw,
			}
		}
	}
}
