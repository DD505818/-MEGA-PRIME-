package main

import "strings"

// CorrelationPosition is the minimal position state needed for portfolio correlation checks.
type CorrelationPosition struct {
	Symbol      string
	Correlation float64
}

// CorrelationDecision describes whether a candidate position is allowed.
type CorrelationDecision struct {
	Allowed            bool    `json:"allowed"`
	Reason             string  `json:"reason"`
	AverageCorrelation float64 `json:"average_correlation"`
	PositionCount      int     `json:"position_count"`
}

// StreamingCorrelationMatrix is a conservative gate facade. A future streaming matrix can replace
// the static candidate-position input while preserving this decision contract.
type StreamingCorrelationMatrix struct {
	MaxAverageCorrelation float64
}

func NewStreamingCorrelationMatrix(_ int, maxAverageCorrelation float64) *StreamingCorrelationMatrix {
	if maxAverageCorrelation <= 0 || maxAverageCorrelation > 1 {
		maxAverageCorrelation = 0.70
	}
	return &StreamingCorrelationMatrix{MaxAverageCorrelation: maxAverageCorrelation}
}

// EvaluateAdd checks whether adding a candidate would breach the average-correlation limit.
// Safety behavior:
// - zero existing positions is allowed
// - one existing position is allowed unless the candidate correlation itself breaches the threshold
// - invalid/missing symbol is rejected
// - correlations are clamped to [0, 1] for conservative positive-correlation gating
func (m *StreamingCorrelationMatrix) EvaluateAdd(candidateSymbol string, positions []CorrelationPosition) CorrelationDecision {
	candidateSymbol = strings.TrimSpace(candidateSymbol)
	if candidateSymbol == "" {
		return CorrelationDecision{Allowed: false, Reason: "INVALID_SYMBOL", PositionCount: len(positions)}
	}

	threshold := 0.70
	if m != nil && m.MaxAverageCorrelation > 0 && m.MaxAverageCorrelation <= 1 {
		threshold = m.MaxAverageCorrelation
	}

	if len(positions) == 0 {
		return CorrelationDecision{Allowed: true, Reason: "NO_EXISTING_POSITIONS", AverageCorrelation: 0, PositionCount: 0}
	}

	sum := 0.0
	count := 0
	for _, position := range positions {
		if strings.EqualFold(strings.TrimSpace(position.Symbol), candidateSymbol) {
			continue
		}
		sum += clamp(position.Correlation, 0, 1)
		count++
	}

	if count == 0 {
		return CorrelationDecision{Allowed: true, Reason: "SAME_SYMBOL_ONLY", AverageCorrelation: 0, PositionCount: len(positions)}
	}

	avg := sum / float64(count)
	if avg > threshold {
		return CorrelationDecision{Allowed: false, Reason: "CORRELATION_LIMIT_EXCEEDED", AverageCorrelation: avg, PositionCount: len(positions)}
	}

	return CorrelationDecision{Allowed: true, Reason: "CORRELATION_WITHIN_LIMIT", AverageCorrelation: avg, PositionCount: len(positions)}
}
