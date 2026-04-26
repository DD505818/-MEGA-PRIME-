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

// EvaluateAdd checks whether adding a candidate would breach the average-correlation limit.
// Safety behavior:
// - zero existing positions is allowed
// - invalid/missing symbol is rejected
// - correlations are clamped to [0, 1] for conservative positive-correlation gating
func (m *StreamingCorrelationMatrix) EvaluateAdd(candidateSymbol string, positions []CorrelationPosition) CorrelationDecision {
	candidateSymbol = strings.TrimSpace(candidateSymbol)
	if candidateSymbol == "" {
		return CorrelationDecision{Allowed: false, Reason: "INVALID_SYMBOL", PositionCount: len(positions)}
	}

	threshold := 0.70
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

// DecodeCorrelationPositions accepts the JSON-unmarshaled signal field
// `correlation_positions`, expected as an array of objects:
// [{"symbol":"BTCUSD", "correlation":0.42}].
// Malformed entries are skipped instead of crashing the risk engine.
func DecodeCorrelationPositions(raw interface{}) []CorrelationPosition {
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}

	positions := make([]CorrelationPosition, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		symbol, _ := entry["symbol"].(string)
		correlation, ok := entry["correlation"].(float64)
		if strings.TrimSpace(symbol) == "" || !ok {
			continue
		}
		positions = append(positions, CorrelationPosition{Symbol: symbol, Correlation: correlation})
	}
	return positions
}
