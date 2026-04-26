package main

import "math"

// LeverageInput is the deterministic state needed to compute a risk multiplier.
// The scaler never authorizes an order by itself; AEGIS hard stops still own approval.
type LeverageInput struct {
	Equity             float64
	PeakEquity         float64
	EWMAVolatility     float64
	ShortTermVolatility float64
}

// LeverageDecision describes the computed leverage multiplier and the guard that produced it.
type LeverageDecision struct {
	Multiplier float64 `json:"multiplier"`
	Reason     string  `json:"reason"`
}

// VolatilityScaler computes a bounded leverage multiplier from volatility and drawdown state.
type VolatilityScaler struct {
	MinLeverage          float64
	MaxLeverage          float64
	TargetVolatility     float64
	SpikeRatioThreshold  float64
	DeRiskDrawdownStart  float64
	DeRiskDrawdownStop   float64
}

func NewVolatilityScaler() *VolatilityScaler {
	return &VolatilityScaler{
		MinLeverage:         1.0,
		MaxLeverage:         3.0,
		TargetVolatility:    0.015,
		SpikeRatioThreshold: 2.0,
		DeRiskDrawdownStart: 0.05,
		DeRiskDrawdownStop:  0.10,
	}
}

// Compute returns a conservative leverage multiplier.
// Safety behavior:
// - invalid/zero equity returns 1x
// - volatility spike returns 1x
// - drawdown at or above DeRiskDrawdownStop returns 1x
// - otherwise, leverage scales inversely to EWMA volatility and is drawdown-adjusted
func (v *VolatilityScaler) Compute(in LeverageInput) LeverageDecision {
	if v == nil {
		return LeverageDecision{Multiplier: 1.0, Reason: "SCALER_UNAVAILABLE"}
	}

	minLev := positiveOrDefault(v.MinLeverage, 1.0)
	maxLev := positiveOrDefault(v.MaxLeverage, 3.0)
	if maxLev < minLev {
		maxLev = minLev
	}

	if !isPositiveFinite(in.Equity) {
		return LeverageDecision{Multiplier: minLev, Reason: "INVALID_EQUITY"}
	}

	if isPositiveFinite(in.EWMAVolatility) && isPositiveFinite(in.ShortTermVolatility) {
		if in.ShortTermVolatility/in.EWMAVolatility > positiveOrDefault(v.SpikeRatioThreshold, 2.0) {
			return LeverageDecision{Multiplier: minLev, Reason: "VOLATILITY_SPIKE_GUARD"}
		}
	}

	drawdown := 0.0
	if isPositiveFinite(in.PeakEquity) && in.PeakEquity > in.Equity {
		drawdown = (in.PeakEquity - in.Equity) / in.PeakEquity
	}

	stop := positiveOrDefault(v.DeRiskDrawdownStop, 0.10)
	start := positiveOrDefault(v.DeRiskDrawdownStart, 0.05)
	if stop < start {
		stop = start
	}
	if drawdown >= stop {
		return LeverageDecision{Multiplier: minLev, Reason: "DRAWDOWN_DERISK_GUARD"}
	}

	multiplier := maxLev
	if isPositiveFinite(in.EWMAVolatility) {
		multiplier = positiveOrDefault(v.TargetVolatility, 0.015) / in.EWMAVolatility
	}
	multiplier = clamp(multiplier, minLev, maxLev)

	if drawdown > start && stop > start {
		remaining := 1.0 - ((drawdown - start) / (stop - start))
		multiplier = minLev + (multiplier-minLev)*clamp(remaining, 0.0, 1.0)
		return LeverageDecision{Multiplier: clamp(multiplier, minLev, maxLev), Reason: "DRAWDOWN_ADJUSTED"}
	}

	return LeverageDecision{Multiplier: multiplier, Reason: "VOLATILITY_TARGETED"}
}

func isPositiveFinite(v float64) bool {
	return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func positiveOrDefault(v, fallback float64) float64 {
	if isPositiveFinite(v) {
		return v
	}
	return fallback
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
