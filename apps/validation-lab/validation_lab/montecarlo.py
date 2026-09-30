"""Monte Carlo: stress the candidate against simulated return paths.

Two generators, both calibrated to the observed strategy returns:
1. fat_tailed: iid draws from a Student-t fit (df, loc, scale via MLE).
   Gaussian simulations understate tail risk; real returns are leptokurtic.
2. regime_switching: a 2-state Markov chain over volatility regimes (calm /
   stressed, split at median rolling volatility), each with its own Student-t
   fit and an estimated transition matrix.

Metrics per generator: distribution of simulated Sharpes and total returns,
P(sim Sharpe >= observed), P(total return < 0), 5th-percentile outcomes.
"""

import numpy as np
from scipy import stats

from .metrics import sharpe, total_return


def fit_student_t(returns):
    r = np.asarray(returns, dtype=float)
    r = r[np.isfinite(r)]
    df, loc, scale = stats.t.fit(r)
    return {"df": float(df), "loc": float(loc), "scale": float(scale)}


def simulate_fat_tailed(returns, n_paths=2000, seed=0):
    rng = np.random.default_rng(seed)
    r = np.asarray(returns, dtype=float)
    n = r.size
    fit = fit_student_t(r)
    sims = stats.t.rvs(fit["df"], loc=fit["loc"], scale=fit["scale"],
                       size=(n_paths, n), random_state=rng)
    return sims, {"generator": "student_t", **fit}


def _vol_regimes(returns, window=24 * 7):
    r = np.asarray(returns, dtype=float)
    vol = np.empty(r.size)
    for i in range(r.size):
        seg = r[max(0, i - window + 1):i + 1]
        vol[i] = seg.std(ddof=1) if seg.size > 1 else 0.0
    vol = np.where(np.isfinite(vol), vol, 0.0)
    return (vol >= np.median(vol)).astype(int)  # 0 = calm, 1 = stressed


def simulate_regime_switching(returns, n_paths=2000, seed=0, window=24 * 7):
    rng = np.random.default_rng(seed)
    r = np.asarray(returns, dtype=float)
    n = r.size
    regimes = _vol_regimes(r, window)
    fits = {}
    for state in (0, 1):
        seg = r[regimes == state]
        fits[state] = fit_student_t(seg) if seg.size > 30 else fit_student_t(r)
    # transition matrix with Laplace smoothing
    trans = np.ones((2, 2))
    for a, b in zip(regimes[:-1], regimes[1:]):
        trans[a, b] += 1
    trans = trans / trans.sum(axis=1, keepdims=True)
    p00, p11 = trans[0, 0], trans[1, 1]
    p_start0 = float(np.mean(regimes == 0))
    # vectorized Markov chain: states (n_paths, n)
    u = rng.random((n_paths, n))
    states = np.empty((n_paths, n), dtype=np.int64)
    states[:, 0] = (rng.random(n_paths) >= p_start0).astype(np.int64)
    for i in range(1, n):
        prev = states[:, i - 1]
        stay = np.where(prev == 0, p00, p11)
        states[:, i] = np.where(u[:, i] < stay, prev, 1 - prev)
    f0, f1 = fits[0], fits[1]
    sims = stats.t.rvs(
        np.where(states == 0, f0["df"], f1["df"]),
        loc=np.where(states == 0, f0["loc"], f1["loc"]),
        scale=np.where(states == 0, f0["scale"], f1["scale"]),
        random_state=rng)
    return sims, {"generator": "regime_switching_2state",
                  "transition_matrix": trans.tolist(),
                  "fits": fits, "window": window}


def summarize_paths(sims, observed_returns, periods_per_year=24 * 365):
    obs_sr = sharpe(observed_returns, periods_per_year)
    obs_tr = total_return(observed_returns)
    sim_sr = np.array([sharpe(p, periods_per_year) for p in sims])
    sim_tr = np.array([total_return(p) for p in sims])
    return {
        "n_paths": int(sims.shape[0]),
        "path_length": int(sims.shape[1]),
        "observed_sharpe": obs_sr,
        "observed_total_return": obs_tr,
        "sim_sharpe_mean": float(sim_sr.mean()),
        "sim_sharpe_p5": float(np.percentile(sim_sr, 5)),
        "sim_sharpe_p95": float(np.percentile(sim_sr, 95)),
        "prob_sim_sharpe_ge_observed": float(np.mean(sim_sr >= obs_sr)),
        "prob_total_return_negative": float(np.mean(sim_tr < 0)),
        "sim_total_return_p5": float(np.percentile(sim_tr, 5)),
        "sim_total_return_median": float(np.median(sim_tr)),
    }


def run(returns, n_paths=2000, seed=0, periods_per_year=24 * 365):
    out = {}
    sims, meta = simulate_fat_tailed(returns, n_paths, seed)
    out["fat_tailed"] = {**meta, **summarize_paths(sims, returns, periods_per_year)}
    sims, meta = simulate_regime_switching(returns, n_paths, seed + 1)
    out["regime_switching"] = {**meta, **summarize_paths(sims, returns, periods_per_year)}
    return out
