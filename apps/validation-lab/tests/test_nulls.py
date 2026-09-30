import numpy as np

from validation_lab import nulls


def _trending(n=3000, seed=1):
    """AR(1) asset with momentum edge: position = sign(prev return).

    The edge lives in *timing* (alignment of positions with autocorrelated
    returns), which shuffling destroys -- unlike a perfect-foresight fixture
    whose all-positive returns are permutation-invariant.
    """
    rng = np.random.default_rng(seed)
    asset = np.empty(n)
    asset[0] = rng.normal(0, 0.01)
    for t in range(1, n):
        asset[t] = 0.25 * asset[t - 1] + rng.normal(0, 0.01)
    pos = np.zeros(n)
    pos[:-1] = np.sign(asset[:-1])
    strat = np.zeros(n)
    strat[1:] = pos[:-1] * asset[1:]
    return asset, pos, strat


def test_permutation_detects_edge():
    _, _, strat = _trending()
    out = nulls.permutation_test(strat, n_perm=200, seed=0)
    assert out["p_value"] < 0.05


def test_permutation_no_false_positive_on_noise():
    rng = np.random.default_rng(7)
    noise = rng.normal(0, 0.01, size=3000)
    out = nulls.permutation_test(noise, n_perm=200, seed=0)
    assert out["p_value"] > 0.01  # noise should not look significant


def test_random_timing_detects_edge():
    asset, pos, _ = _trending()
    out = nulls.random_timing_test(asset, pos, n_perm=200, seed=0)
    assert out["p_value"] < 0.05


def test_deterministic():
    _, _, strat = _trending()
    a = nulls.permutation_test(strat, n_perm=100, seed=3)
    b = nulls.permutation_test(strat, n_perm=100, seed=3)
    assert a["p_value"] == b["p_value"]
