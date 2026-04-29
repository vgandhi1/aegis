package processor

import (
	"context"
	"testing"
	"time"

	"aegis/correlation-worker/internal/models"
)

type mockState struct {
	vin, fw string
}

func (m *mockState) GetCurrentState(stationID string) (string, string) {
	if stationID == "5" {
		return m.vin, m.fw
	}
	return "", ""
}

func TestStreamEnricher_Run(t *testing.T) {
	ms := &mockState{vin: "VIN123", fw: "v1"}
	en := NewStreamEnricher(ms)
	in := make(chan models.PLCMessage, 2)
	out := make(chan models.EnrichedMessage, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go en.Run(ctx, in, out)

	in <- models.PLCMessage{StationID: "5", Torque: 10, Timestamp: time.Now().UnixMilli()}
	select {
	case e := <-out:
		if e.VIN != "VIN123" || e.Firmware != "v1" || e.Torque != 10 {
			t.Fatalf("unexpected %+v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestStreamEnricher_DropsIdle(t *testing.T) {
	ms := &mockState{vin: "", fw: ""}
	en := NewStreamEnricher(ms)
	in := make(chan models.PLCMessage, 1)
	out := make(chan models.EnrichedMessage, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go en.Run(ctx, in, out)
	in <- models.PLCMessage{StationID: "5", Torque: 1, Timestamp: 0}
	select {
	case <-out:
		t.Fatal("expected drop")
	case <-time.After(100 * time.Millisecond):
	}
}
