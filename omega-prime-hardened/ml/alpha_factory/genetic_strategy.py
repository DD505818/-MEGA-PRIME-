"""Simple genetic strategy scorer used by alpha_factory service."""

from __future__ import annotations


def fitness(returns: list[float], penalty: float = 0.1) -> float:
    if not returns:
        return 0.0
    mean = sum(returns) / len(returns)
    downside = [r for r in returns if r < 0]
    downside_risk = abs(sum(downside) / len(downside)) if downside else 0.0
    return mean - penalty * downside_risk
