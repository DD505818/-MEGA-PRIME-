"""Bootstrap confidence intervals for Sharpe and annualized return.

Implements the stationary bootstrap (Politis & Romano): resample blocks with
random geometric lengths so the resampled series stays stationary, preserving
short-range dependence that iid resampling would destroy. Also provides the
simpler circular block bootstrap for comparison, and the empirical
contiguous-block path bootstrap (terminal return / drawdown / ruin
distribution), consolidated here from the former `backtests.lab.monte_carlo`
module so this package holds the one canonical bootstrap implementation.
"""

import numpy as np

from .metrics import _clean, annualized_return, sharpe


def _resample_blocks(rng, n, mean_block):
    """Yield index arrays: stationary bootstrap (geometric block lengths)."""
    idx = []
    while len(idx) < n:
        start = int(rng.integers(0, n))
        length = int(rng.geometric(1.0 / mean_block))
        idx.extend((start + j) % n for j in range(length))
    return np.array(idx[:n], dtype=int)


def stationary_bootstrap(returns, n_boot=2000, mean_block=20, seed=0,
                         periods_per_year=24 * 365, ci=0.95):
    rng = np.random.default_rng(seed)
    r = np.asarray(returns, dtype=float)
    n = r.size
    sharpes = np.empty(n_boot)
    annrets = np.empty(n_boot)
    for i in range(n_boot):
        sample = r[_resample_blocks(rng, n, mean_block)]
        sharpes[i] = sharpe(sample, periods_per_year)
        annrets[i] = annualized_return(sample, periods_per_year)
    lo, hi = (1 - ci) / 2 * 100, (1 + ci) / 2 * 100
    return {
        "method": "stationary",
        "n_boot": n_boot,
        "mean_block": mean_block,
        "sharpe_mean": float(sharpes.mean()),
        "sharpe_ci": [float(np.percentile(sharpes, lo)), float(np.percentile(sharpes, hi))],
        "ann_return_mean": float(annrets.mean()),
        "ann_return_ci": [float(np.percentile(annrets, lo)), float(np.percentile(annrets, hi))],
        "ci_level": ci,
    }


def circular_block_bootstrap(returns, n_boot=2000, block=20, seed=0,
                             periods_per_year=24 * 365, ci=0.95):
    rng = np.random.default_rng(seed)
    r = np.asarray(returns, dtype=float)
    n = r.size
    n_blocks = int(np.ceil(n / block))
    sharpes = np.empty(n_boot)
    for i in range(n_boot):
        starts = rng.integers(0, n, size=n_blocks)
        idx = np.concatenate([np.arange(s, s + block) % n for s in starts])[:n]
        sharpes[i] = sharpe(r[idx], periods_per_year)
    lo, hi = (1 - ci) / 2 * 100, (1 + ci) / 2 * 100
    return {
        "method": "circular_block",
        "n_boot": n_boot,
        "block": block,
        "sharpe_mean": float(sharpes.mean()),
        "sharpe_ci": [float(np.percentile(sharpes, lo)), float(np.percentile(sharpes, hi))],
        "ci_level": ci,
    }


def empirical_block_bootstrap(
    values,
    *,
    simulations: int = 1000,
    block_size: int = 20,
    seed: int = 7,
    ruin_drawdown: float = 0.25,
) -> dict:
    """Empirical contiguous-block bootstrap of the return path itself.

    Resamples fixed-length contiguous blocks (no wraparound) to rebuild
    full-length paths, and reports the distribution of terminal return,
    maximum drawdown, and P(max drawdown <= -ruin_drawdown). Unlike the
    Sharpe-CI bootstraps above, this stresses the realized path geometry.
    Strict input validation via `_clean`.
    """
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
