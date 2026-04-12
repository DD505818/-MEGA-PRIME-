"""Impact model helpers."""

from __future__ import annotations


def estimate_slippage(order_size: float, liquidity: float, volatility: float) -> float:
    if liquidity <= 0:
        raise ValueError("liquidity must be positive")
    size_factor = order_size / liquidity
    return max(0.0, size_factor * volatility * 100)
