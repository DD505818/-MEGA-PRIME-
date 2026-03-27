"""Compatibility allocation module used by integration tests."""

from dataclasses import dataclass
from typing import Iterable


@dataclass(frozen=True)
class StrategyStat:
    name: str
    sharpe: float
    drawdown: float
    regime_fit: float


def allocate(stats: Iterable[StrategyStat]) -> dict[str, float]:
    """Allocate normalized weights based on risk-adjusted score.

    Uses a single pass and avoids temporary list allocations for better throughput
    when many strategies are evaluated.
    """
    entries = tuple(stats)
    if not entries:
        return {}

    weights: dict[str, float] = {}
    total = 0.0
    for stat in entries:
        score = max(0.0, stat.sharpe) * (1 - min(0.95, stat.drawdown)) * stat.regime_fit
        weights[stat.name] = score
        total += score

    if total <= 0:
        uniform_weight = 1.0 / len(entries)
        return {stat.name: uniform_weight for stat in entries}

    inv_total = 1.0 / total
    return {name: score * inv_total for name, score in weights.items()}
