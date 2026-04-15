"""Deterministic lightweight predictor used by tests and local backtests."""

from __future__ import annotations


def predict_next(prices: list[float], *, context: int = 20) -> float:
    if len(prices) < 2:
        raise ValueError("at least two prices are required")

    window = prices[-context:]
    slope = (window[-1] - window[0]) / max(1, len(window) - 1)
    forecast = window[-1] + slope
    return round(forecast, 6)
