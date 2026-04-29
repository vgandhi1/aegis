package models

// PLCMessage is raw edge telemetry before MES correlation.
type PLCMessage struct {
	StationID string  `json:"station_id"`
	Torque    float64 `json:"torque"`
	Timestamp int64   `json:"timestamp"`
}

// EnrichedMessage combines PLC readings with active work-order context.
type EnrichedMessage struct {
	PLCMessage
	VIN      string `json:"vin"`
	Firmware string `json:"firmware"`
}
