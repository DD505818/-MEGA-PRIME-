from dataclasses import dataclass
from typing import List

@dataclass
class StrategyStat:
    name: str
    sharpe: float
    drawdown: float
    regime_fit: float


def allocate(stats: List[StrategyStat]) -> dict:
    weights = {}
    total = 0.0
    for s in stats:
        score = max(0.0, s.sharpe) * (1 - min(0.95, s.drawdown)) * s.regime_fit
        weights[s.name] = score
        total += score
    if total == 0:
        n = len(stats) or 1
        return {s.name: 1 / n for s in stats}
    return {k: v / total for k, v in weights.items()}
