"""Simple moving-window forecaster used as baseline predictor."""

from __future__ import annotations


def forecast(values: list[float], window: int = 8) -> float:
    if not values:
        raise ValueError("values required")
    subset = values[-window:] if len(values) >= window else values
    return sum(subset) / len(subset)
