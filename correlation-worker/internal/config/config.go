package config

import (
	"os"
	"strconv"
)

// Env holds runtime configuration from the environment.
type Env struct {
	NATSURL         string
	ClickHouseAddrs []string
	EnrichWorkers   int
}

// Load reads configuration with sensible defaults for local development.
func Load() Env {
	n := os.Getenv("NATS_URL")
	if n == "" {
		n = "nats://127.0.0.1:4222"
	}
	ch := os.Getenv("CLICKHOUSE_ADDR")
	if ch == "" {
		ch = "127.0.0.1:9000"
	}
	w := 5
	if s := os.Getenv("ENRICH_WORKERS"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			w = v
		}
	}
	return Env{
		NATSURL:         n,
		ClickHouseAddrs: []string{ch},
		EnrichWorkers:   w,
	}
}
