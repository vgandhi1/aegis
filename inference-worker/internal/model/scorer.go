package model

import (
	"math"
	"sync"
)

// Config defines anomaly detection parameters.
// In production these are derived from loaded ONNX model metadata.
type Config struct {
	TorqueMeanNm float64
	TorqueStdNm  float64
	Threshold    float64
}

// Scorer is thread-safe and designed for zero-downtime hot-swapping.
// Replace Score() internals with an onnxruntime-go session call in production.
type Scorer struct {
	mu  sync.RWMutex
	cfg Config
}

func New(cfg Config) *Scorer {
	return &Scorer{cfg: cfg}
}

// Swap atomically replaces the scorer config under a write lock.
// Call this after downloading and loading a new ONNX file from the model registry.
func (s *Scorer) Swap(cfg Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
}

// Score returns an anomaly probability in [0,1] for a batch of torque readings.
// Uses a sigmoid-of-z-score heuristic: ~0.12 at 1σ, ~0.5 at 2σ, ~0.98 at 4σ.
func (s *Scorer) Score(torqueReadings []float64) float64 {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	if len(torqueReadings) == 0 {
		return 0
	}
	var maxZ float64
	for _, t := range torqueReadings {
		if z := math.Abs(t-cfg.TorqueMeanNm) / cfg.TorqueStdNm; z > maxZ {
			maxZ = z
		}
	}
	return 1.0 / (1.0 + math.Exp(-(maxZ - 2.0)))
}

func (s *Scorer) Threshold() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Threshold
}
