
import numpy as np

from backtests.lab.statistics import (
    candidate_metrics,
    cscv_probability_of_backtest_overfitting,
    deflated_sharpe_probability,
)


def test_candidate_statistics_are_bounded_and_deterministic():
    rng = np.random.default_rng(4)
    matrix = rng.normal(0.0002, 0.01, size=(160, 4))
    metrics = candidate_metrics(matrix, ["a", "b", "c", "d"], periods_per_year=252)
    assert set(metrics) == {"a", "b", "c", "d"}
    pbo = cscv_probability_of_backtest_overfitting(matrix, slices=8)
    assert 0.0 <= pbo["pbo"] <= 1.0
    assert pbo["paths"] == 70

    sharpes = [metrics[k]["sharpe"] / np.sqrt(252) for k in metrics]
    best = max(range(4), key=lambda i: sharpes[i])
    dsr = deflated_sharpe_probability(matrix[:, best], sharpes)
    assert 0.0 <= dsr["deflated_sharpe_probability"] <= 1.0
