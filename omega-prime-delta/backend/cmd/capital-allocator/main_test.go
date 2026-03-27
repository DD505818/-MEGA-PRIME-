package main

import "testing"

func TestApplyWeightUpdateNormalizesWeights(t *testing.T) {
	update := map[string]float64{"Agent001": 2, "Agent002": 1}
	applyWeightUpdate(update)

	mu.RLock()
	defer mu.RUnlock()

	total := 0.0
	for _, w := range strategyWeights {
		total += w
	}
	if total < 0.999999 || total > 1.000001 {
		t.Fatalf("expected normalized total of 1, got %f", total)
	}

	if strategyWeights["Agent001"] <= strategyWeights["Agent002"] {
		t.Fatalf("expected Agent001 weight to exceed Agent002 after update")
	}
}
