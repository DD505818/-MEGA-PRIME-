import numpy as np

from validation_lab import montecarlo


def test_fat_tailed_recovers_df():
    rng = np.random.default_rng(0)
    from scipy import stats
    r = stats.t.rvs(5, loc=0.001, scale=0.01, size=5000, random_state=rng)
    fit = montecarlo.fit_student_t(r)
    assert 3.0 < fit["df"] < 8.0  # heavy tails recovered, not Gaussian


def test_simulation_shapes():
    rng = np.random.default_rng(1)
    r = rng.normal(0.0005, 0.01, size=500)
    sims, meta = montecarlo.simulate_fat_tailed(r, n_paths=50, seed=0)
    assert sims.shape == (50, 500)
    assert meta["generator"] == "student_t"


def test_regime_paths():
    rng = np.random.default_rng(2)
    r = np.concatenate([rng.normal(0, 0.005, 500), rng.normal(0, 0.02, 500)])
    sims, meta = montecarlo.simulate_regime_switching(r, n_paths=20, seed=0)
    assert sims.shape == (20, 1000)
    T = np.array(meta["transition_matrix"])
    assert T.shape == (2, 2)
    assert np.allclose(T.sum(axis=1), 1.0)


def test_summarize_keys():
    rng = np.random.default_rng(3)
    r = rng.normal(0.001, 0.01, size=800)
    out = montecarlo.run(r, n_paths=50, seed=0)
    for gen in ("fat_tailed", "regime_switching"):
        s = out[gen]
        assert 0.0 <= s["prob_total_return_negative"] <= 1.0
        assert s["sim_sharpe_p5"] <= s["sim_sharpe_p95"]
