package stream

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"aegis/correlation-worker/internal/models"
	"aegis/correlation-worker/internal/state"

	"github.com/nats-io/nats.go"
)

const (
	StreamName = "AEGIS"
	SubjectPLC = "aegis.telemetry.raw"
	SubjectMES = "aegis.mes.state"
)

// mesPayload matches JSON published by mes-service.
type mesPayload struct {
	StationID string `json:"station_id"`
	VIN       string `json:"vin"`
	Firmware  string `json:"firmware"`
}

// EnsureAegisStream creates the JetStream stream if missing.
func EnsureAegisStream(js nats.JetStreamContext) error {
	_, err := js.StreamInfo(StreamName)
	if err == nil {
		return nil
	}
	if err != nats.ErrStreamNotFound {
		return err
	}
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     StreamName,
		Subjects: []string{"aegis.>"},
		Storage:  nats.FileStorage,
		MaxAge:   24 * time.Hour,
	})
	return err
}

// ListenMES subscribes to MES state updates and updates the cache.
func ListenMES(_ context.Context, nc *nats.Conn, cache *state.StationCache) (*nats.Subscription, error) {
	return nc.Subscribe(SubjectMES, func(msg *nats.Msg) {
		var p mesPayload
		if err := json.Unmarshal(msg.Data, &p); err != nil {
			log.Printf("mes json: %v", err)
			return
		}
		cache.UpdateState(p.StationID, p.VIN, p.Firmware)
		log.Printf("MES cache: station=%s vin=%s fw=%s", p.StationID, p.VIN, p.Firmware)
	})
}

// SubscribePLC starts a JetStream consumer and forwards PLC messages to out.
func SubscribePLC(ctx context.Context, js nats.JetStreamContext, out chan<- models.PLCMessage) (*nats.Subscription, error) {
	return js.Subscribe(SubjectPLC, func(msg *nats.Msg) {
		var m models.PLCMessage
		if err := json.Unmarshal(msg.Data, &m); err != nil {
			log.Printf("plc json: %v", err)
			_ = msg.Nak()
			return
		}
		select {
		case out <- m:
			_ = msg.Ack()
		case <-ctx.Done():
			_ = msg.Nak()
		}
	},
		nats.BindStream(StreamName),
		nats.Durable("aegis-correlation-plc"),
		nats.ManualAck(),
	)
}
