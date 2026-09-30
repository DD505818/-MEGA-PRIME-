"""Shared performance metrics. Pure functions, deterministic."""

import numpy as np
import pandas as pd

TRADING_PERIODS_1H = 24 * 365
TRADING_PERIODS_1D = 365


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
