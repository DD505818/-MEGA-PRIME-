package allocator

import (
	"math"
)

// NormalizeWeights rescales arbitrary scores into a portfolio weight vector
// that sums to 1.0 while preserving relative score proportions.
func NormalizeWeights(scores []float64) []float64 {
	if len(scores) == 0 {
		return []float64{}
	}

	total := 0.0
	for _, s := range scores {
		if s > 0 {
			total += s
		}
	}

	weights := make([]float64, len(scores))
	if total == 0 {
		equal := 1.0 / float64(len(scores))
		for i := range weights {
			weights[i] = equal
		}
		return weights
	}

	for i, s := range scores {
		if s <= 0 {
			continue
		}
		weights[i] = s / total
	}

	// Correct floating-point drift by adjusting the largest bucket.
	sum := 0.0
	largest := 0
	for i, w := range weights {
		sum += w
		if w > weights[largest] {
			largest = i
		}
	}
	weights[largest] += 1 - sum
	weights[largest] = math.Max(0, weights[largest])
	return weights
}
