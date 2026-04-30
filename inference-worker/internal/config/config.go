package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	NATSURL           string
	ModelPollInterval time.Duration
	AnomalyThreshold  float64
}

func Load() Config {
	threshold := 0.95
	if v := os.Getenv("ANOMALY_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			threshold = f
		}
	}
	pollSecs := 300
	if v := os.Getenv("MODEL_POLL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pollSecs = n
		}
	}
	return Config{
		NATSURL:           getenv("NATS_URL", "nats://127.0.0.1:4222"),
		ModelPollInterval: time.Duration(pollSecs) * time.Second,
		AnomalyThreshold:  threshold,
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
