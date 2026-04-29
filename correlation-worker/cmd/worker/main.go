package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"aegis/correlation-worker/internal/config"
	"aegis/correlation-worker/internal/models"
	"aegis/correlation-worker/internal/processor"
	"aegis/correlation-worker/internal/sink"
	"aegis/correlation-worker/internal/state"
	"aegis/correlation-worker/internal/stream"

	"github.com/nats-io/nats.go"
)

func main() {
	log.Println("Starting Aegis Correlation Worker...")
	cfg := config.Load()

	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		log.Fatalf("nats: %v", err)
	}
	defer nc.Drain()

	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("jetstream: %v", err)
	}
	if err := stream.EnsureAegisStream(js); err != nil {
		log.Fatalf("stream: %v", err)
	}

	cache := state.NewStationCache()
	enricher := processor.NewStreamEnricher(cache)

	inStream := make(chan models.PLCMessage, 4096)
	outStream := make(chan models.EnrichedMessage, 4096)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, err := stream.ListenMES(ctx, nc, cache); err != nil {
		log.Fatalf("mes subscribe: %v", err)
	}
	if _, err := stream.SubscribePLC(ctx, js, inStream); err != nil {
		log.Fatalf("plc subscribe: %v", err)
	}

	chWriter, err := sink.NewEnrichedWriter(ctx, cfg.ClickHouseAddrs)
	if err != nil {
		log.Fatalf("clickhouse: %v", err)
	}
	defer chWriter.Close()

	go chWriter.Run(ctx, outStream)

	for i := 0; i < cfg.EnrichWorkers; i++ {
		go enricher.Run(ctx, inStream, outStream)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")
	cancel()
}
