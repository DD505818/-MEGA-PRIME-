"""Compatibility shim.

Canonical implementations: `validation_lab.metrics` (return statistics)
and `validation_lab.overfit` (multiple-testing / CSCV statistics).
`sharpe_ratio` and `max_drawdown` keep the historical strict-validation
contract (raise on invalid simple returns) by cleaning before delegating
to the canonical lenient metrics.
"""

from validation_lab.metrics import (
    _clean,
    candidate_metrics,
    profit_factor,
    sortino_ratio,
)
from validation_lab.metrics import max_drawdown as _canonical_max_drawdown
from validation_lab.metrics import sharpe as _canonical_sharpe
from validation_lab.overfit import (
    cscv_probability_of_backtest_overfitting,
    deflated_sharpe_probability,
    probabilistic_sharpe_probability,
)


def sharpe_ratio(values, *, periods_per_year: int = 1) -> float:
    return _canonical_sharpe(_clean(values), periods_per_year=periods_per_year)


def max_drawdown(values) -> float:
    return _canonical_max_drawdown(_clean(values))


__all__ = [
    "candidate_metrics",
    "cscv_probability_of_backtest_overfitting",
    "deflated_sharpe_probability",
    "max_drawdown",
    "probabilistic_sharpe_probability",
    "profit_factor",
    "sharpe_ratio",
    "sortino_ratio",
]
