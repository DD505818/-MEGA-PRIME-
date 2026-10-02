"""MC v2 (validation protocol v2) fixture + property tests.

Discipline: these fixtures were written BEFORE the MC v2 engine and the
engine was developed against them. No test here touches real campaign data
(#56 or any candidate); the frozen #56 regression is a separate, one-shot,
post-freeze step, not a development loop.

Engine contract under test:
  montecarlo.run_v2(returns, n_paths=..., seed=..., periods_per_year=...,
                    max_loss_prob=0.20) -> dict with
    "protocol_version" == "v2", "engine_version", "path" in {"dense","sparse"},
    "path_selection" (predetermined diagnostics), "decision" with
    "mc_verdict" in {"PASS","FAIL","INDETERMINATE"} and finite
    "prob_total_return_negative", and "diagnostics" persisted.
"""

import numpy as np
import pytest

from validation_lab import montecarlo

PPY = 8760
N_PATHS = 200  # modest: fixtures must run in seconds


def _sparse_series(rng, n, active_prob, active_mean, active_sd,
                   block_len=8, gap_len=20):
    """Clustered sparse returns: alternating inactive gaps and active blocks."""
    r = np.zeros(n)
    t = 0
    while t < n:
        t += int(rng.integers(max(1, gap_len // 2), gap_len * 2))
        if t >= n:
            break
        b = int(rng.integers(max(1, block_len // 2), block_len * 2))
        seg = slice(t, min(n, t + b))
        if seg.stop > seg.start:
            r[seg] = rng.normal(active_mean, active_sd, seg.stop - seg.start)
        t += b
    return r


# ---------------- dense path fixtures ----------------

def test_dense_iid_positive_edge_stable_finite():
    rng = np.random.default_rng(11)
    r = rng.normal(0.0015, 0.01, 1500)
    out = montecarlo.run_v2(r, n_paths=N_PATHS, seed=7, periods_per_year=PPY)
    assert out["protocol_version"] == "v2"
    assert out["path"] == "dense"
    d = out["decision"]
    assert d["mc_verdict"] in ("PASS", "FAIL")  # never INDETERMINATE here
    assert np.isfinite(d["prob_total_return_negative"])
    assert 0.0 <= d["prob_total_return_negative"] <= 1.0
    assert d["prob_total_return_negative"] <= 0.20  # real edge passes
    assert d["mc_verdict"] == "PASS"
    # fit diagnostics persisted
    assert 2.1 <= out["diagnostics"]["regime_switching"]["fits"]["0"]["fit"]["df"] <= 1e12


def test_dense_heavy_tail_edge_fat_tail_stable():
    rng = np.random.default_rng(12)
    from scipy import stats
    r = stats.t.rvs(3, loc=0.0012, scale=0.008, size=1500, random_state=rng)
    out = montecarlo.run_v2(r, n_paths=N_PATHS, seed=7, periods_per_year=PPY)
    assert out["path"] == "dense"
    d = out["decision"]
    assert d["mc_verdict"] in ("PASS", "FAIL")
    assert np.isfinite(d["prob_total_return_negative"])
    fit = out["diagnostics"]["fat_tailed"]["fit"]
    assert 2.1 <= fit["df"] <= 1e12  # heavy tails recovered, sane


def test_dense_zero_edge_high_failure_probability():
    rng = np.random.default_rng(13)
    r = rng.normal(0.0, 0.01, 1500)
    out = montecarlo.run_v2(r, n_paths=N_PATHS, seed=7, periods_per_year=PPY)
    assert out["path"] == "dense"
    d = out["decision"]
    assert d["mc_verdict"] == "FAIL"
    assert d["prob_total_return_negative"] > 0.20


# ---------------- sparse path fixtures ----------------

def test_sparse_profitable_process_recovers_signal():
    rng = np.random.default_rng(21)
    r = _sparse_series(rng, 1500, 0.3, 0.004, 0.02)
    out = montecarlo.run_v2(r, n_paths=N_PATHS, seed=7, periods_per_year=PPY)
    assert out["path"] == "sparse", out["path_selection"]
    d = out["decision"]
    assert d["mc_verdict"] == "PASS"
    assert np.isfinite(d["prob_total_return_negative"])
    assert d["prob_total_return_negative"] <= 0.20


def test_sparse_losing_process_rejected():
    rng = np.random.default_rng(22)
    r = _sparse_series(rng, 1500, 0.3, -0.004, 0.02)
    out = montecarlo.run_v2(r, n_paths=N_PATHS, seed=7, periods_per_year=PPY)
    assert out["path"] == "sparse"
    assert out["decision"]["mc_verdict"] == "FAIL"
    assert out["decision"]["prob_total_return_negative"] > 0.20


def test_sparse_random_timing_no_artificial_edge():
    rng = np.random.default_rng(23)
    r = _sparse_series(rng, 1500, 0.3, 0.0, 0.02)  # zero-mean active noise
    out = montecarlo.run_v2(r, n_paths=600, seed=7, periods_per_year=PPY)
    assert out["path"] == "sparse"
    assert out["decision"]["mc_verdict"] == "FAIL"  # must not invent an edge


def test_clustered_trades_dependence_retained():
    rng = np.random.default_rng(24)
    r = _sparse_series(rng, 1500, 0.3, 0.004, 0.02, block_len=12, gap_len=24)
    out = montecarlo.run_v2(r, n_paths=N_PATHS, seed=7, periods_per_year=PPY)
    assert out["path"] == "sparse"
    diag = out["diagnostics"]["sparse_event"]
    obs = diag["observed_mean_active_run"]
    sim = diag["sim_mean_active_run"]
    assert obs > 1.5  # fixture really is clustered
    assert 0.5 * obs <= sim <= 2.0 * obs  # simulated paths keep the clustering


def test_regime_dependent_edge_regime_structure_retained():
    rng = np.random.default_rng(25)
    n = 1500
    # calm first half (positive drift), stressed second half (zero drift, high vol)
    r = np.concatenate([rng.normal(0.002, 0.008, 750),
                        rng.normal(0.0, 0.03, 750)])
    out = montecarlo.run_v2(r, n_paths=N_PATHS, seed=7, periods_per_year=PPY)
    assert out["path"] == "dense"  # no zeros -> dense path
    diag = out["diagnostics"]["regime_switching"]
    assert abs(diag["sim_calm_frac"] - diag["observed_calm_frac"]) < 0.10


def test_long_inactivity_runs_zero_mass_handled():
    rng = np.random.default_rng(26)
    r = _sparse_series(rng, 1500, 0.05, 0.01, 0.03, block_len=6, gap_len=120)
    assert float(np.mean(r == 0)) > 0.9
    out = montecarlo.run_v2(r, n_paths=N_PATHS, seed=7, periods_per_year=PPY)
    assert out["path"] == "sparse"
    d = out["decision"]
    assert d["mc_verdict"] in ("PASS", "FAIL", "INDETERMINATE")
    assert np.isfinite(d["prob_total_return_negative"])


def test_too_few_trades_indeterminate():
    rng = np.random.default_rng(27)
    r = np.zeros(1500)
    idx = rng.choice(1500, 15, replace=False)
    r[idx] = rng.normal(0.01, 0.02, 15)
    out = montecarlo.run_v2(r, n_paths=N_PATHS, seed=7, periods_per_year=PPY)
    assert out["path"] == "sparse"
    d = out["decision"]
    assert d["mc_verdict"] == "INDETERMINATE"
    assert any("nonzero" in reason for reason in d["indeterminate_reasons"])


def test_pathological_tail_sample_controlled_failure_no_nan():
    rng = np.random.default_rng(28)
    # dense by the zero-fraction rule, but the two-scale mixture collapses
    # the Student-t MLE (df < 2.1) -> INDETERMINATE, never NaN propagation
    r = rng.normal(0, 1e-12, 1500)
    spikes = rng.choice(1500, 50, replace=False)
    r[spikes] = rng.choice([-1.0, 1.0], 50) * rng.uniform(0.3, 0.8, 50)
    out = montecarlo.run_v2(r, n_paths=N_PATHS, seed=7, periods_per_year=PPY)
    d = out["decision"]
    assert d["mc_verdict"] == "INDETERMINATE"
    assert d["prob_total_return_negative"] is None  # no number is fabricated
    # no NaN anywhere in the decision payload
    import json
    assert "NaN" not in json.dumps(out["decision"], allow_nan=False)


# ---------------- property invariants ----------------

def test_fixed_seed_identical_results():
    rng = np.random.default_rng(31)
    r = _sparse_series(rng, 1200, 0.3, 0.004, 0.02)
    a = montecarlo.run_v2(r, n_paths=N_PATHS, seed=99, periods_per_year=PPY)
    b = montecarlo.run_v2(r, n_paths=N_PATHS, seed=99, periods_per_year=PPY)
    assert (a["decision"]["prob_total_return_negative"]
            == b["decision"]["prob_total_return_negative"])
    assert a["decision"]["mc_verdict"] == b["decision"]["mc_verdict"]
    rng2 = np.random.default_rng(32)
    r2 = rng2.normal(0.001, 0.01, 1200)
    c = montecarlo.run_v2(r2, n_paths=N_PATHS, seed=99, periods_per_year=PPY)
    d = montecarlo.run_v2(r2, n_paths=N_PATHS, seed=99, periods_per_year=PPY)
    assert (c["decision"]["prob_total_return_negative"]
            == d["decision"]["prob_total_return_negative"])


def test_permutation_of_dense_iid_is_predictable():
    rng = np.random.default_rng(33)
    r = rng.normal(0.0015, 0.01, 1200)
    perm = rng.permutation(len(r))
    a = montecarlo.run_v2(r, n_paths=N_PATHS, seed=5, periods_per_year=PPY)
    b = montecarlo.run_v2(r[perm], n_paths=N_PATHS, seed=5, periods_per_year=PPY)
    # t-fit is order-invariant on the same multiset -> identical simulations
    assert (a["decision"]["prob_total_return_negative"]
            == b["decision"]["prob_total_return_negative"])
    assert a["decision"]["mc_verdict"] == b["decision"]["mc_verdict"]


def test_adding_costs_cannot_improve_simulated_returns():
    rng = np.random.default_rng(34)
    r = _sparse_series(rng, 1200, 0.3, 0.004, 0.02)
    active = (np.abs(r) > 0).astype(float)
    drag1 = 0.0005 * active
    drag2 = 0.0020 * active

    def _p(x):
        return montecarlo.run_v2(x, n_paths=600, seed=5,
                                 periods_per_year=PPY)["decision"][
                                     "prob_total_return_negative"]
    p0, p1, p2 = _p(r), _p(r - drag1), _p(r - drag2)
    assert p1 >= p0 - 1e-9
    assert p2 >= p1 - 1e-9


def test_nans_cannot_reach_gate_decision():
    rng = np.random.default_rng(35)
    r = _sparse_series(rng, 1200, 0.3, 0.004, 0.02)
    r[::97] = np.nan
    r[::211] = np.inf
    out = montecarlo.run_v2(r, n_paths=N_PATHS, seed=5, periods_per_year=PPY)
    d = out["decision"]
    assert np.isfinite(d["prob_total_return_negative"])
    assert d["mc_verdict"] in ("PASS", "FAIL", "INDETERMINATE")
    import json
    json.dumps(d, allow_nan=False)  # raises on NaN/Infinity


def test_fit_diagnostics_persisted_with_artifact():
    rng = np.random.default_rng(36)
    r = _sparse_series(rng, 1200, 0.3, 0.004, 0.02)
    out = montecarlo.run_v2(r, n_paths=N_PATHS, seed=5, periods_per_year=PPY)
    diag = out["diagnostics"]["sparse_event"]
    for key in ("n", "n_nonzero", "zero_frac", "n_active_blocks",
                "activity_transition_matrix", "regime_transition_matrix",
                "seed_split_prob", "second_estimator_prob",
                "observed_mean_active_run", "sim_mean_active_run"):
        assert key in diag, key
    rng2 = np.random.default_rng(37)
    r2 = rng2.normal(0.001, 0.01, 1200)
    out2 = montecarlo.run_v2(r2, n_paths=N_PATHS, seed=5, periods_per_year=PPY)
    assert "fat_tailed" in out2["diagnostics"]
    assert "regime_switching" in out2["diagnostics"]


def test_sparse_verdict_keys_shape():
    rng = np.random.default_rng(38)
    r = _sparse_series(rng, 800, 0.3, 0.004, 0.02)
    out = montecarlo.run_v2(r, n_paths=100, seed=5, periods_per_year=PPY)
    assert out["decision"]["generator"] == "sparse_event"
    assert "sparse_event" in out
    assert "sparse_block_check" in out
    assert out["sparse_event"]["prob_total_return_negative"] is not None
