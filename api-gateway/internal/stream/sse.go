package stream

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
)

// AlertsSSE streams defect alerts from NATS to the browser using Server-Sent Events.
// The React dashboard connects here to receive live anomaly notifications without polling.
func AlertsSSE(nc *nats.Conn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		msgCh := make(chan *nats.Msg, 32)
		sub, err := nc.ChanSubscribe("aegis.defect.alerts", msgCh)
		if err != nil {
			log.Printf("sse: nats subscribe: %v", err)
			http.Error(w, "subscription error", http.StatusInternalServerError)
			return
		}
		defer sub.Unsubscribe()

		hb := time.NewTicker(30 * time.Second)
		defer hb.Stop()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-hb.C:
				fmt.Fprintf(w, ": heartbeat\n\n")
				flusher.Flush()
			case msg := <-msgCh:
				var data interface{}
				if err := json.Unmarshal(msg.Data, &data); err != nil {
					continue
				}
				payload, _ := json.Marshal(data)
				fmt.Fprintf(w, "data: %s\n\n", payload)
				flusher.Flush()
			}
		}
	}
}
