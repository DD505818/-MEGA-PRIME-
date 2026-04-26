package main

import "testing"

func TestVolatilityScalerInvalidEquityReturnsOneX(t *testing.T) {
	scaler := NewVolatilityScaler()
	decision := scaler.Compute(LeverageInput{Equity: 0, PeakEquity: 10000, EWMAVolatility: 0.005})

	if decision.Multiplier != 1.0 {
		t.Fatalf("expected 1x for invalid equity, got %.4f", decision.Multiplier)
	}
	if decision.Reason != "INVALID_EQUITY" {
		t.Fatalf("expected INVALID_EQUITY, got %s", decision.Reason)
	}
}

func TestVolatilityScalerCapsAtMaxLeverageInLowVol(t *testing.T) {
	scaler := NewVolatilityScaler()
	decision := scaler.Compute(LeverageInput{Equity: 10000, PeakEquity: 10000, EWMAVolatility: 0.001})

	if decision.Multiplier != 3.0 {
		t.Fatalf("expected max leverage 3x, got %.4f", decision.Multiplier)
	}
	if decision.Reason != "VOLATILITY_TARGETED" {
		t.Fatalf("expected VOLATILITY_TARGETED, got %s", decision.Reason)
	}
}

func TestVolatilityScalerSpikeGuardReturnsOneX(t *testing.T) {
	scaler := NewVolatilityScaler()
	decision := scaler.Compute(LeverageInput{
		Equity:              10000,
		PeakEquity:          10000,
		EWMAVolatility:      0.01,
		ShortTermVolatility: 0.025,
	})

	if decision.Multiplier != 1.0 {
		t.Fatalf("expected 1x during volatility spike, got %.4f", decision.Multiplier)
	}
	if decision.Reason != "VOLATILITY_SPIKE_GUARD" {
		t.Fatalf("expected VOLATILITY_SPIKE_GUARD, got %s", decision.Reason)
	}
}

func TestVolatilityScalerDrawdownStopReturnsOneX(t *testing.T) {
	scaler := NewVolatilityScaler()
	decision := scaler.Compute(LeverageInput{Equity: 9000, PeakEquity: 10000, EWMAVolatility: 0.001})

	if decision.Multiplier != 1.0 {
		t.Fatalf("expected 1x at drawdown stop, got %.4f", decision.Multiplier)
	}
	if decision.Reason != "DRAWDOWN_DERISK_GUARD" {
		t.Fatalf("expected DRAWDOWN_DERISK_GUARD, got %s", decision.Reason)
	}
}

func TestVolatilityScalerDrawdownAdjustsBetweenStartAndStop(t *testing.T) {
	scaler := NewVolatilityScaler()
	decision := scaler.Compute(LeverageInput{Equity: 9300, PeakEquity: 10000, EWMAVolatility: 0.001})

	if decision.Multiplier <= 1.0 || decision.Multiplier >= 3.0 {
		t.Fatalf("expected leverage between 1x and 3x under drawdown adjustment, got %.4f", decision.Multiplier)
	}
	if decision.Reason != "DRAWDOWN_ADJUSTED" {
		t.Fatalf("expected DRAWDOWN_ADJUSTED, got %s", decision.Reason)
	}
}
