"""Market impact model primitives."""

from __future__ import annotations


def estimate_slippage(qty: float, volatility: float, liquidity: float, coeff: float = 0.0005) -> float:
    if qty <= 0:
        raise ValueError("qty must be positive")
    if liquidity <= 0:
        raise ValueError("liquidity must be positive")
    if volatility < 0:
        raise ValueError("volatility cannot be negative")

    slippage = coeff * (qty / liquidity) * (1 + volatility)
    return round(min(0.1, max(0.0, slippage)), 6)
