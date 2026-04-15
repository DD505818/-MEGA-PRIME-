"""Simple genetic strategy helper used by alpha factory pipelines."""

from __future__ import annotations

from dataclasses import dataclass
from random import Random


@dataclass(frozen=True)
class StrategyGenome:
    momentum: float
    mean_reversion: float
    volatility_target: float


def fitness(genome: StrategyGenome, pnl: float, drawdown: float) -> float:
    """Compute a bounded fitness score from pnl and risk metrics."""
    risk_penalty = max(0.0, drawdown) * (1 + genome.volatility_target)
    raw = pnl * (1 + genome.momentum) - risk_penalty * (1 + genome.mean_reversion)
    return max(-1.0, min(1.0, raw))


def mutate(genome: StrategyGenome, *, seed: int | None = None) -> StrategyGenome:
    rng = Random(seed)
    return StrategyGenome(
        momentum=max(0.0, genome.momentum + rng.uniform(-0.05, 0.05)),
        mean_reversion=max(0.0, genome.mean_reversion + rng.uniform(-0.05, 0.05)),
        volatility_target=max(0.01, genome.volatility_target + rng.uniform(-0.02, 0.02)),
    )
