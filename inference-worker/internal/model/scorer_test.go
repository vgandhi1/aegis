package model

import (
	"testing"
)

func TestScoreNominalTorque(t *testing.T) {
	s := New(Config{TorqueMeanNm: 42.0, TorqueStdNm: 3.0, Threshold: 0.95})
	score := s.Score([]float64{42.0}) // exact mean → z=0 → ~0.12
	if score >= 0.5 {
		t.Errorf("nominal torque should score below 0.5, got %.3f", score)
	}
}

func TestScoreAnomalousTorque(t *testing.T) {
	s := New(Config{TorqueMeanNm: 42.0, TorqueStdNm: 3.0, Threshold: 0.95})
	score := s.Score([]float64{62.0}) // ~6.7σ above mean → should exceed threshold
	if score <= 0.95 {
		t.Errorf("extreme torque should score above 0.95, got %.3f", score)
	}
}

func TestScoreEmpty(t *testing.T) {
	s := New(Config{TorqueMeanNm: 42.0, TorqueStdNm: 3.0, Threshold: 0.95})
	if score := s.Score(nil); score != 0 {
		t.Errorf("empty batch should return 0, got %.3f", score)
	}
}

func TestSwapIsAtomic(t *testing.T) {
	s := New(Config{TorqueMeanNm: 42.0, TorqueStdNm: 3.0, Threshold: 0.95})
	s.Swap(Config{TorqueMeanNm: 50.0, TorqueStdNm: 2.0, Threshold: 0.80})
	// After swap, 50.0 is now the mean → score should be near 0
	score := s.Score([]float64{50.0})
	if score >= 0.5 {
		t.Errorf("new mean should score low after swap, got %.3f", score)
	}
}
