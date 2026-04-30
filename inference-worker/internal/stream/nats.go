package stream

import (
	"context"
	"encoding/json"
	"log"

	"aegis/inference-worker/internal/model"

	"github.com/nats-io/nats.go"
)

const (
	SubjectPLC    = "aegis.telemetry.raw"
	SubjectAlerts = "aegis.defect.alerts"
	StreamName    = "AEGIS"
)

type plcMessage struct {
	StationID string  `json:"station_id"`
	Torque    float64 `json:"torque"`
	Timestamp int64   `json:"timestamp"`
}

// DefectAlert is published to SubjectAlerts when the scorer flags an anomaly.
type DefectAlert struct {
	StationID       string  `json:"station_id"`
	AnomalyScore    float64 `json:"anomaly_score"`
	TriggerTorqueNm float64 `json:"trigger_torque_nm"`
	Timestamp       int64   `json:"timestamp"`
}

// Worker subscribes to PLC telemetry and publishes scored defect alerts.
type Worker struct {
	nc     *nats.Conn
	scorer *model.Scorer
}

func NewWorker(nc *nats.Conn, scorer *model.Scorer) *Worker {
	return &Worker{nc: nc, scorer: scorer}
}

// Subscribe binds the worker to the AEGIS JetStream with a durable consumer.
func (w *Worker) Subscribe(_ context.Context, js nats.JetStreamContext) (*nats.Subscription, error) {
	return js.Subscribe(SubjectPLC, w.handle,
		nats.BindStream(StreamName),
		nats.Durable("aegis-inference-plc"),
		nats.ManualAck(),
	)
}

func (w *Worker) handle(msg *nats.Msg) {
	var m plcMessage
	if err := json.Unmarshal(msg.Data, &m); err != nil {
		log.Printf("inference: bad plc json: %v", err)
		_ = msg.Nak()
		return
	}
	// Fail-open: ack before scoring so factory throughput is never blocked by ML.
	_ = msg.Ack()

	score := w.scorer.Score([]float64{m.Torque})
	if score > w.scorer.Threshold() {
		w.publishAlert(m, score)
	}
}

func (w *Worker) publishAlert(m plcMessage, score float64) {
	alert := DefectAlert{
		StationID:       m.StationID,
		AnomalyScore:    score,
		TriggerTorqueNm: m.Torque,
		Timestamp:       m.Timestamp,
	}
	payload, err := json.Marshal(alert)
	if err != nil {
		log.Printf("inference: marshal alert: %v", err)
		return
	}
	if err := w.nc.Publish(SubjectAlerts, payload); err != nil {
		log.Printf("inference: publish alert: %v", err)
		return
	}
	log.Printf("DEFECT ALERT station=%s score=%.3f torque=%.1fNm", m.StationID, score, m.Torque)
}
