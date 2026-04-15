package allocator

import (
	"errors"
	"math"
)

type Signal struct {
	Name  string
	Score float64
}

type Allocation struct {
	Name   string
	Weight float64
	Amount float64
}

func AllocateCapital(capital float64, signals []Signal, maxWeight float64) ([]Allocation, error) {
	if capital <= 0 {
		return nil, errors.New("capital must be positive")
	}
	if len(signals) == 0 {
		return nil, errors.New("signals must not be empty")
	}
	if maxWeight <= 0 || maxWeight > 1 {
		return nil, errors.New("maxWeight must be in (0,1]")
	}

	totalScore := 0.0
	for _, signal := range signals {
		if signal.Score > 0 {
			totalScore += signal.Score
		}
	}
	if totalScore == 0 {
		return nil, errors.New("at least one signal score must be positive")
	}

	allocs := make([]Allocation, 0, len(signals))
	remaining := 1.0
	for _, signal := range signals {
		base := math.Max(0, signal.Score) / totalScore
		weight := math.Min(base, maxWeight)
		if weight > remaining {
			weight = remaining
		}
		remaining -= weight
		allocs = append(allocs, Allocation{
			Name:   signal.Name,
			Weight: weight,
			Amount: capital * weight,
		})
	}

	if remaining > 0 {
		allocs[0].Weight += remaining
		allocs[0].Amount = capital * allocs[0].Weight
	}

	return allocs, nil
}
