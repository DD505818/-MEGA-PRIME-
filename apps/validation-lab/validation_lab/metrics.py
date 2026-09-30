"""Shared performance metrics. Pure functions, deterministic.

Canonical home for the low-level return statistics shared by the gauntlet
and the research tooling (fold generation, CSCV/PBO evaluation, sealed
snapshot analysis). The research-grade helpers (`_clean`, `sortino_ratio`,
`profit_factor`, `candidate_metrics`) were consolidated here from the
former `backtests.lab.statistics` module; they keep their strict
input-validation contract (raise on invalid simple returns).
"""

import numpy as np
import pandas as pd

TRADING_PERIODS_1H = 24 * 365
TRADING_PERIODS_1D = 365


def _clean(values) -> np.ndarray:
    """Validate a simple-return series for research statistics.

    Raises ValueError unless the input is 1-D, has >= 3 finite
    observations, and every return is > -1.
    """
    arr = np.asarray(values, dtype=float)
    if arr.ndim != 1:
        raise ValueError("returns must be one-dimensional")
    if len(arr) < 3 or not np.isfinite(arr).all():
        raise ValueError("returns must contain at least 3 finite observations")
    if (arr <= -1).any():
        raise ValueError("simple returns must be greater than -1")
    return arr


def sharpe(returns, periods_per_year=TRADING_PERIODS_1H):
    """Annualized Sharpe ratio (risk-free = 0). Returns 0.0 on degenerate input."""
    r = np.asarray(returns, dtype=float)
    r = r[np.isfinite(r)]
    if r.size < 2:
        return 0.0
    std = r.std(ddof=1)
    if std == 0 or not np.isfinite(std):
        return 0.0
    return float(r.mean() / std * np.sqrt(periods_per_year))


def total_return(returns):
    r = np.asarray(returns, dtype=float)
    r = r[np.isfinite(r)]
    if r.size == 0:
        return 0.0
    return float(np.prod(1.0 + r) - 1.0)


def max_drawdown(returns):
    r = np.asarray(returns, dtype=float)
    r = r[np.isfinite(r)]
    if r.size == 0:
        return 0.0
    equity = np.cumprod(1.0 + r)
    peak = np.maximum.accumulate(equity)
    dd = (equity - peak) / peak
    return float(dd.min())


def annualized_return(returns, periods_per_year=TRADING_PERIODS_1H):
    r = np.asarray(returns, dtype=float)
    r = r[np.isfinite(r)]
    if r.size == 0:
        return 0.0
    return float(np.prod(1.0 + r) ** (periods_per_year / r.size) - 1.0)


def strategy_returns(df, positions):
    """Causal strategy returns: position held during bar t is positions[t-1].

    positions: Series/DataFrame column in {-1, 0, 1}, aligned to df index.
    No lookahead: signal at bar t can only use data up to bar t.
    """
    rets = df["close"].pct_change().fillna(0.0)
    pos = pd.Series(positions, index=df.index).fillna(0.0).clip(-1, 1)
    return (pos.shift(1).fillna(0.0) * rets).astype(float)


def sortino_ratio(values, *, periods_per_year: int = 1) -> float:
    """Annualized Sortino ratio (strict input validation via `_clean`)."""
    r = _clean(values)
    downside = r[r < 0]
    if len(downside) < 2:
        return float("inf") if float(np.mean(r)) > 0 else 0.0
    dd = float(np.std(downside, ddof=1))
    if dd == 0:
        return 0.0
    return float(np.mean(r) / dd * np.sqrt(periods_per_year))


def profit_factor(values) -> float:
    """Gross gains / gross losses (strict input validation via `_clean`)."""
    r = _clean(values)
    gains = float(r[r > 0].sum())
    losses = float(-r[r < 0].sum())
    if losses == 0:
        return float("inf") if gains > 0 else 0.0
    return gains / losses


def candidate_metrics(returns_matrix, candidate_names: list[str], *, periods_per_year: int) -> dict[str, dict[str, float | None]]:
    """Per-candidate metric table for a [time, candidates] return matrix."""
    matrix = np.asarray(returns_matrix, dtype=float)
    if matrix.ndim != 2 or matrix.shape[1] != len(candidate_names):
        raise ValueError("candidate_names must match returns matrix columns")
    out: dict[str, dict[str, float | None]] = {}
    for idx, name in enumerate(candidate_names):
        r = _clean(matrix[:, idx])
        sortino = sortino_ratio(r, periods_per_year=periods_per_year)
        pf = profit_factor(r)
        out[name] = {
            "sharpe": sharpe(r, periods_per_year),
            "sortino": sortino if np.isfinite(sortino) else None,
            "max_drawdown": max_drawdown(r),
            "profit_factor": pf if np.isfinite(pf) else None,
            "hit_rate": float(np.mean(r > 0)),
            "total_return": total_return(r),
        }
    return out
