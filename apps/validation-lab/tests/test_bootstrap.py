import numpy as np

from validation_lab import bootstrap


def test_ci_covers_true_sharpe():
    rng = np.random.default_rng(0)
    r = rng.normal(0.001, 0.01, size=3000)
    out = bootstrap.stationary_bootstrap(r, n_boot=300, mean_block=20, seed=0)
    lo, hi = out["sharpe_ci"]
    assert lo < hi
    # true Sharpe = 0.001/0.01*sqrt(24*365) ~ 9.36; CI should be positive
    assert lo > 0


def test_ci_wide_on_noise():
    rng = np.random.default_rng(1)
    r = rng.normal(0, 0.01, size=3000)
    out = bootstrap.stationary_bootstrap(r, n_boot=300, mean_block=20, seed=0)
    lo, hi = out["sharpe_ci"]
    assert lo < 0 < hi  # noise CI straddles zero


def test_circular_block_runs():
    rng = np.random.default_rng(2)
    r = rng.normal(0.0005, 0.01, size=1000)
    out = bootstrap.circular_block_bootstrap(r, n_boot=100, block=25, seed=0)
    assert out["sharpe_ci"][0] < out["sharpe_ci"][1]


def test_deterministic():
    rng = np.random.default_rng(3)
    r = rng.normal(0.001, 0.01, size=1000)
    a = bootstrap.stationary_bootstrap(r, n_boot=100, seed=5)
    b = bootstrap.stationary_bootstrap(r, n_boot=100, seed=5)
    assert a["sharpe_ci"] == b["sharpe_ci"]
