import numpy as np
import pandas as pd
import pytest

from validation_lab import overfit


def _df(n=5000, seed=0, drift=0.001):
    rng = np.random.default_rng(seed)
    px = 100 * np.cumprod(1 + drift + rng.normal(0, 0.01, size=n))
    return pd.DataFrame({"close": px},
                        index=pd.date_range("2023-01-01", periods=n, freq="h",
                                            tz="UTC"))


def test_dsr_calibrated_at_null():
    # DSR is P(true SR > 0); at observed == expected null Sharpe it must be 0.5
    trials = list(np.linspace(-0.5, 0.5, 49)) + [1.5]
    _, sr0 = overfit.deflated_sharpe_ratio(1.5, trials, 500)
    dsr, _ = overfit.deflated_sharpe_ratio(sr0, trials, 500)
    assert abs(dsr - 0.5) < 1e-9


def test_textbook_dsr_matches_per_period_anchor():
    # Analysis anchor: SR 0.60, K=9, trial sd 0.15, Kraken 1h dev n=15316.
    base = np.arange(9, dtype=float)
    u = (base - base.mean()) / base.std(ddof=1)
    trials = list(1.0 + 0.15 * u)
    dsr, _ = overfit.textbook_deflated_sharpe_ratio(
        0.60, trials, 15316, 8760, skew=-1.0, kurt=8.0)
    assert abs(dsr - 0.688) < 0.01


def test_dsr_penalizes_trial_count():
    # same best Sharpe from more trials -> strictly lower DSR
    def dsr_for(k):
        trials = list(np.linspace(-0.5, 0.5, k - 1)) + [1.0]
        dsr, _ = overfit.deflated_sharpe_ratio(1.0, trials, n_obs=300)
        return dsr

    assert dsr_for(500) < dsr_for(20)


def test_min_backtest_length_grows_for_small_sr():
    a = overfit.min_backtest_length(2.0)
    b = overfit.min_backtest_length(0.5)
    assert b > a > 0


def test_sensitivity_survives_robust():
    df = _df()

    def factory(p):
        # long-only constant: insensitive to params
        return pd.Series(1.0, index=df.index)

    out = overfit.parameter_sensitivity(factory, {"a": 10, "b": 5.0}, df)
    assert out["survived"]
    assert len(out["trial_sharpes"]) == 1 + 2 * 2


def test_sensitivity_fails_fragile():
    df = _df()

    def factory(p):
        # sign flips when 'a' is perturbed down 20% (10 -> 8)
        sign = 1.0 if p["a"] >= 9 else -1.0
        return pd.Series(sign, index=df.index)

    out = overfit.parameter_sensitivity(factory, {"a": 10}, df)
    assert not out["survived"]


def test_dsr_needs_two_trials():
    with pytest.raises(ValueError):
        overfit.deflated_sharpe_ratio(1.0, [1.0], 100)
