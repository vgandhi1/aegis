# Correlation Worker Module Layout

## Project Directory Structure

```text
correlation-worker/
├── cmd/
│   └── worker/
│       └── main.go           # Application entrypoint; wires up dependencies
├── internal/
│   ├── models/
│   │   └── telemetry.go      # Data structures (PLCMessage, EnrichedMessage)
│   ├── state/
│   │   ├── cache.go          # The RWMutex MES state manager
│   │   └── cache_test.go     # Tests verifying concurrency and race conditions
│   ├── stream/
│   │   └── nats.go           # NATS JetStream consumer/publisher implementations
│   └── processor/
│       ├── enricher.go       # The core stream-table join business logic
│       └── enricher_test.go  # Unit tests using mock streams and state
├── go.mod
├── go.sum
└── Makefile                  # Build, test, and linting commands
```

## Module Implementation Details

### 1. `internal/state/cache.go`
This package isolates the state and the mutex, allowing for aggressive concurrent testing against it without needing a real NATS connection.

```go
package state

import (
	"sync"
)

// StationCache holds the slow-moving MES state.
type StationCache struct {
	mu          sync.RWMutex
	currentVIN  string
	firmwareVer string
}

func NewStationCache() *StationCache {
	return &StationCache{}
}

// UpdateState applies a new MES configuration.
func (c *StationCache) UpdateState(vin, firmware string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentVIN = vin
	c.firmwareVer = firmware
}

// GetCurrentState safely retrieves the active VIN and firmware.
func (c *StationCache) GetCurrentState() (vin, firmware string) {
	c.mu.RLock()
	// Multiple readers can proceed in parallel here, blocking only if a write is occurring.
	defer c.mu.RUnlock()
	return c.currentVIN, c.firmwareVer
}
```

### 2. `internal/processor/enricher.go`
The processor contains the pure business logic. It takes an input channel, queries the state, and writes to an output channel, making it highly unit-testable.

```go
package processor

import (
	"context"
	"correlation-worker/internal/models"
)

// StateReader defines the interface for fetching enterprise context.
type StateReader interface {
	GetCurrentState() (vin, firmware string)
}

// StreamEnricher handles the joining of high-frequency data with state.
type StreamEnricher struct {
	stateCache StateReader
}

func NewStreamEnricher(cache StateReader) *StreamEnricher {
	return &StreamEnricher{
		stateCache: cache,
	}
}

// Run processes the incoming telemetry stream until the context is canceled.
func (e *StreamEnricher) Run(ctx context.Context, in <-chan models.PLCMessage, out chan<- models.EnrichedMessage) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-in:
			if !ok {
				return // Input channel closed
			}

			vin, fw := e.stateCache.GetCurrentState()

			// Drop or handle idle telemetry if no vehicle is present
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
```

### 3. `cmd/worker/main.go`
The orchestrator. It initializes the cache, connects to NATS, wires the channels together, and handles graceful shutdowns.

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"correlation-worker/internal/models"
	"correlation-worker/internal/processor"
	"correlation-worker/internal/state"
	// "correlation-worker/internal/stream"
)

func main() {
	log.Println("Starting Aegis Correlation Worker...")

	// 1. Initialize State Cache
	cache := state.NewStationCache()

	// 2. Initialize Core Processor
	enricher := processor.NewStreamEnricher(cache)

	// 3. Setup Channels (Buffered to handle backpressure)
	inStream := make(chan models.PLCMessage, 1000)
	outStream := make(chan models.EnrichedMessage, 1000)

	// 4. Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// (Mocking the NATS connections for illustration)
	// go stream.StartNATSProducer(ctx, outStream)
	// go stream.StartNATSConsumer(ctx, inStream)
	// go stream.ListenForMESUpdates(ctx, cache)

	// 5. Start the worker pool (e.g., 5 concurrent enrichers)
	for i := 0; i < 5; i++ {
		go enricher.Run(ctx, inStream, outStream)
	}

	// 6. Block until termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gracefully...")
	cancel()
	// Allow time for streams to drain in a real implementation
}
```