"""Null-hypothesis tests: is the observed performance distinguishable from luck?

Three baselines, each reporting a p-value (fraction of null Sharpes >= observed):
1. permutation: shuffle the strategy return series (destroys timing).
2. random_timing: circularly shift the position series (preserves position
   autocorrelation, destroys alignment with returns).
3. whitenoise: compare against buy-and-hold via the same permutation machinery.
"""

import numpy as np

from .metrics import sharpe


def _pvalue(observed, null_stats):
    null_stats = np.asarray(null_stats, dtype=float)
    return float((1.0 + np.sum(null_stats >= observed)) / (1.0 + null_stats.size))


def permutation_test(returns, n_perm=2000, seed=0, periods_per_year=24 * 365):
    """Sign-flip (Rademacher) permutation test; p = P(null Sharpe >= observed).

    NOTE: naive shuffling of the return series is vacuous for Sharpe -- a
    permutation preserves the multiset, hence mean and std, hence Sharpe
    exactly (p would always be 1). The valid permutation null for a Sharpe
    statistic randomly flips the SIGN of each period's return: under H0
    (zero-mean, symmetric returns) every sign pattern is equally likely,
    while a real edge (persistent sign alignment) is destroyed.
    """
    rng = np.random.default_rng(seed)
    r = np.asarray(returns, dtype=float)
    obs = sharpe(r, periods_per_year)
    nulls = np.empty(n_perm)
    for i in range(n_perm):
        signs = rng.choice([-1.0, 1.0], size=r.size)
        nulls[i] = sharpe(r * signs, periods_per_year)
    return {"observed_sharpe": obs, "p_value": _pvalue(obs, nulls),
            "null_mean": float(nulls.mean()), "null_std": float(nulls.std()),
            "n_perm": n_perm, "method": "sign_flip"}


def random_timing_test(asset_returns, positions, n_perm=2000, seed=0,
                       periods_per_year=24 * 365):
    """Circularly shift positions; preserves turnover structure, breaks timing."""
    rng = np.random.default_rng(seed)
    ar = np.asarray(asset_returns, dtype=float)
    pos = np.asarray(positions, dtype=float)
    n = ar.size
    # proper causal strategy returns for the unshifted series
    base = np.zeros(n)
    base[1:] = pos[:-1] * ar[1:]
    obs = sharpe(base, periods_per_year)
    nulls = np.empty(n_perm)
    for i in range(n_perm):
        shift = int(rng.integers(1, n))
        sp = np.roll(pos, shift)
        s = np.zeros(n)
        s[1:] = sp[:-1] * ar[1:]
        nulls[i] = sharpe(s, periods_per_year)
    return {"observed_sharpe": obs, "p_value": _pvalue(obs, nulls),
            "null_mean": float(nulls.mean()), "null_std": float(nulls.std()),
            "n_perm": n_perm}


def buy_and_hold_comparison(asset_returns, strategy_returns, n_perm=2000, seed=0,
                            periods_per_year=24 * 365):
    """Is the strategy's Sharpe distinguishable from buy-and-hold's?

    Permutes the paired (strategy, BH) return differences; p = P(|null diff|
    >= |observed diff|) two-sided on the Sharpe gap.
    """
    rng = np.random.default_rng(seed)
    s = np.asarray(strategy_returns, dtype=float)
    b = np.asarray(asset_returns, dtype=float)
    obs_gap = sharpe(s, periods_per_year) - sharpe(b, periods_per_year)
    diffs = s - b
    nulls = np.empty(n_perm)
    for i in range(n_perm):
        signs = rng.choice([-1.0, 1.0], size=diffs.size)
        nulls[i] = sharpe(diffs * signs, periods_per_year)
    p = float((1.0 + np.sum(np.abs(nulls) >= abs(obs_gap))) / (1.0 + n_perm))
    return {"strategy_sharpe": sharpe(s, periods_per_year),
            "buy_hold_sharpe": sharpe(b, periods_per_year),
            "sharpe_gap": obs_gap, "p_value": p, "n_perm": n_perm}
