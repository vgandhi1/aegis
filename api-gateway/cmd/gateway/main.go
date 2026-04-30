package main

import (
	"log"
	"net/http"

	"aegis/api-gateway/internal/config"
	gqlserver "aegis/api-gateway/internal/graphql"
	"aegis/api-gateway/internal/rest"
	"aegis/api-gateway/internal/stream"

	"github.com/nats-io/nats.go"
)

func main() {
	log.Println("Starting Aegis API Gateway...")
	cfg := config.Load()

	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		log.Fatalf("nats: %v", err)
	}
	defer nc.Drain()

	schema, err := gqlserver.BuildSchema(cfg.ClickHouseHTTP)
	if err != nil {
		log.Fatalf("graphql schema: %v", err)
	}

	h := rest.NewHandler(nc, cfg.JWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/token", h.Token)
	mux.HandleFunc("POST /api/v1/webhooks/mes", h.MESWebhook)
	mux.HandleFunc("GET /graphql", gqlserver.Handler(schema))
	mux.HandleFunc("POST /graphql", gqlserver.Handler(schema))
	mux.HandleFunc("GET /api/v1/alerts/stream", stream.AlertsSSE(nc))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"api-gateway"}`))
	})

	log.Printf("API Gateway listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, mux); err != nil {
		log.Fatal(err)
	}
}
