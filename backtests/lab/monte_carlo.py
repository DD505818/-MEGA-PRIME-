
from __future__ import annotations

from typing import Any

import numpy as np

from .statistics import _clean


def empirical_block_bootstrap(
    values,
    *,
    simulations: int = 1000,
    block_size: int = 20,
    seed: int = 7,
    ruin_drawdown: float = 0.25,
) -> dict[str, Any]:
    returns = _clean(values)
    n = len(returns)
    if simulations <= 0:
        raise ValueError("simulations must be positive")
    if block_size <= 0 or block_size > n:
        raise ValueError("block_size must be in [1, len(returns)]")
    if not 0 < ruin_drawdown < 1:
        raise ValueError("ruin_drawdown must be between 0 and 1")

    rng = np.random.default_rng(seed)
    max_start = n - block_size + 1
    terminal = np.empty(simulations, dtype=float)
    max_dd = np.empty(simulations, dtype=float)

    for i in range(simulations):
        chunks = []
        total = 0
        while total < n:
            start = int(rng.integers(0, max_start))
            chunk = returns[start : start + block_size]
            chunks.append(chunk)
            total += len(chunk)
        path = np.concatenate(chunks)[:n]
        equity = np.cumprod(1.0 + path)
        peaks = np.maximum.accumulate(equity)
        drawdowns = equity / peaks - 1.0
        terminal[i] = equity[-1] - 1.0
        max_dd[i] = float(drawdowns.min())

    q = lambda arr, p: float(np.quantile(arr, p))
    return {
        "method": "empirical_contiguous_block_bootstrap",
        "seed": seed,
        "simulations": simulations,
        "block_size": block_size,
        "terminal_return": {
            "p05": q(terminal, 0.05),
            "p50": q(terminal, 0.50),
            "p95": q(terminal, 0.95),
        },
        "max_drawdown": {
            "p05": q(max_dd, 0.05),
            "p50": q(max_dd, 0.50),
            "p95": q(max_dd, 0.95),
        },
        "ruin_drawdown": ruin_drawdown,
        "ruin_probability": float(np.mean(max_dd <= -ruin_drawdown)),
    }
