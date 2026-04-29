package state

import "sync"

type stationEntry struct {
	vin      string
	firmware string
}

// StationCache holds slow-moving MES state keyed by station ID.
type StationCache struct {
	mu       sync.RWMutex
	stations map[string]stationEntry
}

// NewStationCache returns an empty multi-station cache.
func NewStationCache() *StationCache {
	return &StationCache{
		stations: make(map[string]stationEntry),
	}
}

// UpdateState applies a new MES session for a station.
func (c *StationCache) UpdateState(stationID, vin, firmware string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stations == nil {
		c.stations = make(map[string]stationEntry)
	}
	c.stations[stationID] = stationEntry{vin: vin, firmware: firmware}
}

// GetCurrentState returns the active VIN and firmware for a station.
func (c *StationCache) GetCurrentState(stationID string) (vin, firmware string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if e, ok := c.stations[stationID]; ok {
		return e.vin, e.firmware
	}
	return "", ""
}
