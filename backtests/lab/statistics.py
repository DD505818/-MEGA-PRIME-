
from __future__ import annotations

import itertools
import math
from statistics import NormalDist
from typing import Any

import numpy as np

_EULER_GAMMA = 0.5772156649015329


def _clean(values) -> np.ndarray:
    arr = np.asarray(values, dtype=float)
    if arr.ndim != 1:
        raise ValueError("returns must be one-dimensional")
    if len(arr) < 3 or not np.isfinite(arr).all():
        raise ValueError("returns must contain at least 3 finite observations")
    if (arr <= -1).any():
        raise ValueError("simple returns must be greater than -1")
    return arr


def sharpe_ratio(values, *, periods_per_year: int = 1) -> float:
    r = _clean(values)
    sd = float(np.std(r, ddof=1))
    if sd == 0:
        return 0.0
    return float(np.mean(r) / sd * math.sqrt(periods_per_year))


def sortino_ratio(values, *, periods_per_year: int = 1) -> float:
    r = _clean(values)
    downside = r[r < 0]
    if len(downside) < 2:
        return float("inf") if float(np.mean(r)) > 0 else 0.0
    dd = float(np.std(downside, ddof=1))
    if dd == 0:
        return 0.0
    return float(np.mean(r) / dd * math.sqrt(periods_per_year))


def max_drawdown(values) -> float:
    r = _clean(values)
    equity = np.cumprod(1.0 + r)
    peaks = np.maximum.accumulate(equity)
    drawdowns = equity / peaks - 1.0
    return float(drawdowns.min())


def profit_factor(values) -> float:
    r = _clean(values)
    gains = float(r[r > 0].sum())
    losses = float(-r[r < 0].sum())
    if losses == 0:
        return float("inf") if gains > 0 else 0.0
    return gains / losses


def probabilistic_sharpe_probability(values, *, benchmark_sharpe: float = 0.0) -> float:
    r = _clean(values)
    n = len(r)
    sr = sharpe_ratio(r, periods_per_year=1)
    centered = r - np.mean(r)
    sd = float(np.std(r, ddof=1))
    if sd == 0:
        return 1.0 if sr > benchmark_sharpe else 0.0
    skew = float(np.mean(centered**3) / (sd**3))
    kurt = float(np.mean(centered**4) / (sd**4))
    denom = 1.0 - skew * sr + ((kurt - 1.0) / 4.0) * (sr**2)
    if denom <= 0:
        return 0.0
    z = (sr - benchmark_sharpe) * math.sqrt(n - 1.0) / math.sqrt(denom)
    return float(NormalDist().cdf(z))


def deflated_sharpe_probability(best_returns, candidate_sharpes: list[float]) -> dict[str, float]:
    finite = np.asarray([x for x in candidate_sharpes if math.isfinite(x)], dtype=float)
    trials = int(len(finite))
    if trials <= 1:
        benchmark = 0.0
    else:
        sigma = float(np.std(finite, ddof=1))
        z1 = NormalDist().inv_cdf(1.0 - 1.0 / trials)
        z2 = NormalDist().inv_cdf(1.0 - 1.0 / (trials * math.e))
        benchmark = sigma * ((1.0 - _EULER_GAMMA) * z1 + _EULER_GAMMA * z2)
    return {
        "candidate_trials": float(trials),
        "expected_max_sharpe_under_null": float(benchmark),
        "deflated_sharpe_probability": probabilistic_sharpe_probability(
            best_returns, benchmark_sharpe=benchmark
        ),
    }


def _raw_sharpe_2d(matrix: np.ndarray) -> np.ndarray:
    means = np.mean(matrix, axis=0)
    sds = np.std(matrix, axis=0, ddof=1)
    return np.divide(means, sds, out=np.zeros_like(means), where=sds > 0)


def cscv_probability_of_backtest_overfitting(
    returns_matrix,
    *,
    slices: int = 8,
) -> dict[str, Any]:
    matrix = np.asarray(returns_matrix, dtype=float)
    if matrix.ndim != 2 or matrix.shape[1] < 2:
        raise ValueError("returns_matrix must be [time, >=2 candidates]")
    if not np.isfinite(matrix).all() or (matrix <= -1).any():
        raise ValueError("returns_matrix contains invalid simple returns")
    if slices < 4 or slices % 2:
        raise ValueError("slices must be an even integer >= 4")
    if matrix.shape[0] < slices * 4:
        raise ValueError("not enough observations for requested CSCV slices")

    usable = (matrix.shape[0] // slices) * slices
    blocks = np.array_split(np.arange(usable), slices)
    lambdas: list[float] = []
    all_ids = tuple(range(slices))
    for train_ids in itertools.combinations(all_ids, slices // 2):
        train_set = set(train_ids)
        test_ids = [i for i in all_ids if i not in train_set]
        train_idx = np.concatenate([blocks[i] for i in train_ids])
        test_idx = np.concatenate([blocks[i] for i in test_ids])
        train_scores = _raw_sharpe_2d(matrix[train_idx])
        selected = int(np.argmax(train_scores))
        test_scores = _raw_sharpe_2d(matrix[test_idx])

        order = np.argsort(test_scores, kind="mergesort")
        rank = int(np.where(order == selected)[0][0]) + 1
        omega = rank / (matrix.shape[1] + 1.0)
        lambdas.append(float(math.log(omega / (1.0 - omega))))

    pbo = float(np.mean(np.asarray(lambdas) <= 0.0))
    return {
        "pbo": pbo,
        "paths": len(lambdas),
        "median_logit_rank": float(np.median(lambdas)),
        "slices": slices,
        "usable_observations": usable,
    }


def candidate_metrics(returns_matrix, candidate_names: list[str], *, periods_per_year: int) -> dict[str, dict[str, float | None]]:
    matrix = np.asarray(returns_matrix, dtype=float)
    if matrix.ndim != 2 or matrix.shape[1] != len(candidate_names):
        raise ValueError("candidate_names must match returns matrix columns")
    out: dict[str, dict[str, float | None]] = {}
    for idx, name in enumerate(candidate_names):
        r = matrix[:, idx]
        sortino = sortino_ratio(r, periods_per_year=periods_per_year)
        pf = profit_factor(r)
        out[name] = {
            "sharpe": sharpe_ratio(r, periods_per_year=periods_per_year),
            "sortino": sortino if math.isfinite(sortino) else None,
            "max_drawdown": max_drawdown(r),
            "profit_factor": pf if math.isfinite(pf) else None,
            "hit_rate": float(np.mean(r > 0)),
            "total_return": float(np.prod(1.0 + r) - 1.0),
        }
    return out
