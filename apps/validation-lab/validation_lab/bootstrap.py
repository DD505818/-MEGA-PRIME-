"""Bootstrap confidence intervals for Sharpe and annualized return.

Implements the stationary bootstrap (Politis & Romano): resample blocks with
random geometric lengths so the resampled series stays stationary, preserving
short-range dependence that iid resampling would destroy. Also provides the
simpler circular block bootstrap for comparison.
"""

import numpy as np

from .metrics import annualized_return, sharpe


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
