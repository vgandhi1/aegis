package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aegis/inference-worker/internal/config"
	"aegis/inference-worker/internal/model"
	"aegis/inference-worker/internal/stream"

	"github.com/nats-io/nats.go"
)

func main() {
	log.Println("Starting Aegis Foresight Inference Worker...")
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

	scorer := model.New(model.Config{
		TorqueMeanNm: 42.0,
		TorqueStdNm:  3.0,
		Threshold:    cfg.AnomalyThreshold,
	})

	worker := stream.NewWorker(nc, scorer)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if _, err := worker.Subscribe(ctx, js); err != nil {
		log.Fatalf("subscribe: %v", err)
	}

	go pollModelRegistry(ctx, scorer, cfg.ModelPollInterval)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	log.Println("Foresight inference worker shutting down...")
}

// pollModelRegistry simulates periodic ONNX model reload from S3.
// When a new model version is detected, it downloads it and calls scorer.Swap()
// to hot-swap the model config in memory with zero downtime.
func pollModelRegistry(ctx context.Context, scorer *model.Scorer, interval time.Duration) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			// TODO: fetch newest .onnx from S3/MLflow, reload session, call scorer.Swap().
			log.Println("model registry poll (no-op in dev mode)")
		}
	}
}
