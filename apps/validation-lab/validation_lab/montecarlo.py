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


# ---------------------------------------------------------------------------
# MC v2 (validation protocol v2) — two-path engine.
#
# Motivation: the v1 parametric Student-t estimators degenerate on
# zero-inflated (sparse) return series — e.g. a strategy active 3 of 7 days
# produced df ~ 0.1 and scale ~ 1e-14, i.e. a formally computed gate decision
# carrying almost no statistical information. v2 keeps the v1 parametric
# machinery for dense series (hardened with fit diagnostics) and routes
# sparse series to a regime-aware block/event bootstrap that models
# P(R_t != 0) separately from R_t | R_t != 0.
#
# Path selection is by PREDETERMINED diagnostics computed before any gate
# decision (zero-mass fraction and nonzero observation count); the path is
# never chosen based on which result looks favorable.
#
# Verdicts: PASS / FAIL / INDETERMINATE. INDETERMINATE cannot advance
# (the lab maps it to stage FAIL). The gated quantity and threshold are
# unchanged from v1: P(simulated total return < 0) <= max_loss_prob (0.20).
# ---------------------------------------------------------------------------

MC_ENGINE_VERSION = "2.0.0"

# Predetermined path-selection and sanity constants (fixed in advance;
# documented in the lab README; not tuned per candidate).
SPARSE_ZERO_FRAC_THRESHOLD = 0.20  # zero_frac >= this -> sparse path
MIN_N = 100                        # fewer bars -> INDETERMINATE
MIN_NONZERO = 30                     # fewer nonzero obs -> INDETERMINATE
MIN_ACTIVE_BLOCKS = 5                # fewer active blocks -> INDETERMINATE
T_DF_MIN = 2.1                       # Student-t df below this: infinite variance
T_DF_MAX = 1e12                        # huge df just means "near-Gaussian": healthy
T_SCALE_FLOOR = 1e-10                # scale below this: degenerate fit
SEED_SPLIT_TOL = 0.05                # |p(seed) - p(seed+7919)| above this: INDETERMINATE
ESTIMATOR_DISAGREE_TOL = 0.10        # |p_event - p_fixedblock| above this: INDETERMINATE


def _sanitize(returns):
    """Drop non-finite observations; report how many were dropped."""
    r = np.asarray(returns, dtype=float).ravel()
    finite = np.isfinite(r)
    return r[finite], int(r.size - finite.sum())


def _fit_ok(fit):
    """Predetermined Student-t fit diagnostics. Returns (ok, reason)."""
    try:
        df = float(fit["df"]); loc = float(fit["loc"]); scale = float(fit["scale"])
    except (KeyError, TypeError, ValueError):
        return False, "fit params missing/non-numeric"
    if not (np.isfinite(df) and np.isfinite(loc) and np.isfinite(scale)):
        return False, "nonfinite fit params"
    if not (T_DF_MIN <= df <= T_DF_MAX):
        return False, f"df={df:.4g} outside [{T_DF_MIN}, {T_DF_MAX}]"
    if scale < T_SCALE_FLOOR:
        return False, f"scale={scale:.4g} below floor {T_SCALE_FLOOR:.0e}"
    return True, "ok"


def _summary_finite(summary):
    bad = [k for k, v in summary.items()
           if isinstance(v, float) and not np.isfinite(v)]
    return bad


def _laplace_markov(seq, n_states):
    """Transition matrix with Laplace (+1) smoothing; also start probs."""
    trans = np.ones((n_states, n_states))
    seq = np.asarray(seq, dtype=int)
    for a, b in zip(seq[:-1], seq[1:]):
        if 0 <= a < n_states and 0 <= b < n_states:
            trans[a, b] += 1
    trans = trans / trans.sum(axis=1, keepdims=True)
    counts = np.bincount(seq, minlength=n_states).astype(float)
    start = (counts + 1) / (counts.sum() + n_states)
    return trans, start


def _simulate_markov(trans, start, n, rng):
    states = np.empty(n, dtype=np.int64)
    states[0] = int(rng.choice(len(start), p=start))
    u = rng.random(n)
    for i in range(1, n):
        prev = states[i - 1]
        row = trans[prev]
        states[i] = int(np.searchsorted(np.cumsum(row), u[i], side="left"))
        if states[i] >= len(row):
            states[i] = len(row) - 1
    return states


def _active_blocks(r):
    """Maximal runs of nonzero returns -> list of (start, end, vec)."""
    active = np.abs(r) > 0
    blocks = []
    i, n = 0, r.size
    while i < n:
        if active[i]:
            j = i
            while j < n and active[j]:
                j += 1
            blocks.append((i, j, r[i:j].copy()))
            i = j
        else:
            i += 1
    return blocks


def _inactive_gaps(r):
    active = np.abs(r) > 0
    gaps = []
    i, n = 0, r.size
    while i < n:
        if not active[i]:
            j = i
            while j < n and not active[j]:
                j += 1
            gaps.append(j - i)
            i = j
        else:
            i += 1
    return gaps


def _mean_run_length(binary):
    """Mean length of runs where binary == 1 (0.0 if none)."""
    b = np.asarray(binary, dtype=int)
    if not b.any():
        return 0.0
    diff = np.diff(np.concatenate([[0], b, [0]]))
    starts = np.where(diff == 1)[0]
    ends = np.where(diff == -1)[0]
    return float(np.mean(ends - starts))


def _dense_payload(r, n_paths, seed, ppy):
    """v1 parametric machinery, hardened with predetermined fit diagnostics."""
    ind = []
    diag = {}
    sims_ft, meta_ft = simulate_fat_tailed(r, n_paths, seed)
    ok, reason = _fit_ok(meta_ft)
    diag["fat_tailed"] = {"fit": {k: meta_ft[k] for k in ("df", "loc", "scale")},
                          "fit_ok": bool(ok), "fit_reason": reason}
    sum_ft = summarize_paths(sims_ft, r, ppy)
    bad = _summary_finite(sum_ft)
    if bad:
        ind.append(f"fat_tailed summary nonfinite: {bad}")
    sims_rs, meta_rs = simulate_regime_switching(r, n_paths, seed + 1)
    fits_ok = {}
    for state in (0, 1):
        sok, sreason = _fit_ok(meta_rs["fits"][state])
        fits_ok[str(state)] = {"fit": {k: float(meta_rs["fits"][state][k])
                                       for k in ("df", "loc", "scale")},
                               "fit_ok": bool(sok), "fit_reason": sreason}
        if not sok:
            ind.append(f"regime_switching state-{state} fit failed: {sreason}")
    if not ok:
        ind.append(f"fat_tailed fit failed: {reason}")
    diag["regime_switching"] = {
        "fits": fits_ok,
        "transition_matrix": meta_rs["transition_matrix"],
        "window": meta_rs["window"],
    }
    trans = np.asarray(meta_rs["transition_matrix"], dtype=float)
    p00, p11 = trans[0, 0], trans[1, 1]
    # long-run calm fraction of the fitted chain (diagnostic, not a gate)
    diag["regime_switching"]["sim_calm_frac"] = float(
        (1 - p11) / ((1 - p00) + (1 - p11)))
    sum_rs = summarize_paths(sims_rs, r, ppy)
    bad = _summary_finite(sum_rs)
    if bad:
        ind.append(f"regime_switching summary nonfinite: {bad}")
    # regime occupancy preserved? (diagnostic, not a gate)
    diag["regime_switching"]["observed_calm_frac"] = float(
        np.mean(_vol_regimes(r, meta_rs["window"]) == 0))
    payload = {
        "fat_tailed": {**{k: v for k, v in meta_ft.items()},
                       **sum_ft},
        "regime_switching": {"generator": meta_rs["generator"],
                             "transition_matrix": meta_rs["transition_matrix"],
                             "window": meta_rs["window"],
                             **sum_rs},
    }
    return payload, diag, ind, "regime_switching"


def _sparse_event_paths(r, blocks, gaps, pools, trans_r, start_r,
                        n_paths, n, seed):
    """Event bootstrap: resampled inactive gaps alternate with resampled
    active blocks (regime-conditioned). Durations, within-block dependence,
    signs, tails and costs are preserved exactly; activity frequency is
    preserved in expectation via the empirical gap distribution; the
    simulated per-bar regime path preserves regime occupancy/transitions
    and conditions block selection (volatility-state dependence)."""
    rng = np.random.default_rng(seed)
    gap_arr = np.asarray(gaps, dtype=np.int64)
    sims = np.zeros((n_paths, n))
    for p in range(n_paths):
        Rp = _simulate_markov(trans_r, start_r, n, rng)
        t = 0
        out = sims[p]
        while t < n:
            g = int(gap_arr[rng.integers(gap_arr.size)]) if gap_arr.size else 0
            t += g
            if t >= n:
                break
            reg = int(Rp[t])
            pool = pools[reg] if pools[reg] else (pools[0] + pools[1])
            b = pool[int(rng.integers(len(pool)))]
            L = min(b.size, n - t)
            out[t:t + L] = b[:L]
            t += L
    return sims


def _fixed_block_paths(r, n_paths, seed, block_len):
    """Second (independent) sparse estimator: circular block bootstrap with
    a fixed block length. Used only for the estimator-disagreement check."""
    rng = np.random.default_rng(seed)
    n = r.size
    nb = int(np.ceil(n / block_len))
    starts = rng.integers(0, n, size=(n_paths, nb))
    sims = np.empty((n_paths, n))
    for p in range(n_paths):
        parts = [np.concatenate([r[s:], r[:s]])[:block_len] for s in starts[p]]
        sims[p] = np.concatenate(parts)[:n]
    return sims


def _sparse_payload(r, n_paths, seed, ppy):
    """Regime-aware block/event bootstrap for zero-inflated return series."""
    n = r.size
    ind = []
    diag = {}
    window = max(24, min(168, n // 10))
    regimes = _vol_regimes(r, window)
    blocks = _active_blocks(r)
    gaps = _inactive_gaps(r)
    se = {"n": int(n),
          "n_nonzero": int(np.count_nonzero(r)),
          "zero_frac": float(np.mean(r == 0.0)),
          "n_active_blocks": len(blocks),
          "n_inactive_gaps": len(gaps)}
    if len(blocks) < MIN_ACTIVE_BLOCKS:
        ind.append(f"only {len(blocks)} active blocks < MIN_ACTIVE_BLOCKS={MIN_ACTIVE_BLOCKS}")
    # regime-conditioned block pools (volatility-state dependence)
    pools = {0: [], 1: []}
    for (_s, _e, vec) in blocks:
        reg = int(np.round(np.mean(regimes[_s:_e]))) if _e > _s else 0
        pools[reg].append(vec)
    se["pool_sizes"] = {str(k): len(v) for k, v in pools.items()}
    trans_r, start_r = _laplace_markov(regimes, 2)
    se["regime_transition_matrix"] = trans_r.tolist()
    trans_a, _start_a = _laplace_markov((np.abs(r) > 0).astype(int), 2)
    se["activity_transition_matrix"] = trans_a.tolist()
    se["observed_calm_frac"] = float(np.mean(regimes == 0))
    obs_mean_run = _mean_run_length(np.abs(r) > 0)
    se["observed_mean_active_run"] = obs_mean_run

    summary = None
    payload_block = None
    meta = {"generator": "sparse_event_bootstrap",
            "block_unit": "observed active block (maximal nonzero run)",
            "gap_unit": "resampled inactive run length",
            "regime_window": window}
    if not ind:
        sims = _sparse_event_paths(r, blocks, gaps, pools, trans_r, start_r,
                                   n_paths, n, seed)
        summary = summarize_paths(sims, r, ppy)
        bad = _summary_finite(summary)
        if bad:
            ind.append(f"sparse_event summary nonfinite: {bad}")
            summary = None
        else:
            se["sim_mean_active_run"] = float(
                np.mean([_mean_run_length(np.abs(sims[p]) > 0)
                         for p in range(min(n_paths, 50))]))
            # seed-split convergence check (predetermined tolerance)
            sims_b = _sparse_event_paths(r, blocks, gaps, pools, trans_r,
                                         start_r, n_paths, n, seed + 7919)
            sum_b = summarize_paths(sims_b, r, ppy)
            pb = sum_b["prob_total_return_negative"]
            pa = summary["prob_total_return_negative"]
            se["seed_split_prob"] = float(pb)
            # tolerance is noise-aware: max(predetermined floor,
            # 4 binomial SEs of the MC estimate at this n_paths)
            mc_se = float(np.sqrt(pa * (1.0 - pa) / n_paths)) if n_paths else 0.0
            split_tol = max(SEED_SPLIT_TOL, 4.0 * mc_se)
            if not np.isfinite(pb) or abs(pa - pb) > split_tol:
                ind.append(f"seed-split disagreement |{pa:.4f}-{pb:.4f}| "
                           f"> {split_tol:.4f}")
                summary = None
            else:
                # second-estimator disagreement check (fixed block length)
                med_block = float(np.median([b.size for _, _, b in blocks]))
                L2 = max(24, int(4 * med_block))
                sims_c = _fixed_block_paths(r, n_paths, seed + 31337, L2)
                sum_c = summarize_paths(sims_c, r, ppy)
                pc = sum_c["prob_total_return_negative"]
                se["second_estimator_prob"] = float(pc)
                se["second_estimator_block_len"] = L2
                if not np.isfinite(pc) or abs(pa - pc) > ESTIMATOR_DISAGREE_TOL:
                    ind.append(f"estimator disagreement |{pa:.4f}-{pc:.4f}| "
                               f"> {ESTIMATOR_DISAGREE_TOL}")
                    summary = None
                else:
                    payload_block = {"generator": "fixed_circular_block",
                                     "block_len": L2, **sum_c}
    diag = {"sparse_event": se}
    payload = {"sparse_event": ({**meta, **summary} if summary else
                                {**meta, "prob_total_return_negative": None,
                                 "note": "not simulated: INDETERMINATE"})}
    if payload_block is not None:
        payload["sparse_block_check"] = payload_block
    return payload, diag, ind, "sparse_event"


def run_v2(returns, n_paths=2000, seed=0, periods_per_year=24 * 365,
           max_loss_prob=0.20):
    """MC v2: two-path engine. Returns a JSON-serializable dict.

    Path selection (predetermined, before any gate decision):
      zero_frac >= SPARSE_ZERO_FRAC_THRESHOLD -> "sparse", else "dense".
    Decision: P(simulated total return < 0) <= max_loss_prob -> PASS;
    otherwise FAIL. Any INDETERMINATE trigger -> mc_verdict INDETERMINATE
    (prob None), which the lab maps to stage FAIL (cannot advance).
    """
    r, n_dropped = _sanitize(returns)
    n = r.size
    ind = []
    diag = {"n": int(n), "n_dropped_nonfinite": int(n_dropped),
            "seed": int(seed), "n_paths": int(n_paths),
            "max_loss_prob": float(max_loss_prob)}
    if n_dropped:
        diag["note"] = f"{n_dropped} non-finite observations dropped before MC"
    path_selection = {"sparse_zero_frac_threshold": SPARSE_ZERO_FRAC_THRESHOLD,
                      "min_n": MIN_N, "min_nonzero": MIN_NONZERO,
                      "min_active_blocks": MIN_ACTIVE_BLOCKS}
    path = None
    if n < MIN_N:
        ind.append(f"n={n} < MIN_N={MIN_N}: insufficient observations")
    else:
        zero_frac = float(np.mean(r == 0.0))
        n_nonzero = int(np.count_nonzero(r))
        path = "sparse" if zero_frac >= SPARSE_ZERO_FRAC_THRESHOLD else "dense"
        path_selection.update({"zero_frac": zero_frac,
                               "n_nonzero": n_nonzero,
                               "reason": ("zero-inflated: sparse event bootstrap"
                                          if path == "sparse" else
                                          "dense: hardened parametric MC")})
        if n_nonzero < MIN_NONZERO:
            ind.append(f"n_nonzero={n_nonzero} < MIN_NONZERO={MIN_NONZERO}: "
                       "insufficient nonzero observations")
    payload, d2, ind2, decision_gen = ({}, {}, ind, None)
    if not ind and path == "dense":
        payload, d2, ind2, decision_gen = _dense_payload(r, n_paths, seed,
                                                       periods_per_year)
    elif not ind and path == "sparse":
        payload, d2, ind2, decision_gen = _sparse_payload(r, n_paths, seed,
                                                        periods_per_year)
    ind = ind + ind2
    diag.update(d2)
    prob = None
    if not ind and decision_gen:
        prob = payload[decision_gen]["prob_total_return_negative"]
        if prob is None or not np.isfinite(prob):
            ind.append("decision quantity nonfinite")
            prob = None
    verdict = ("INDETERMINATE" if ind
               else ("PASS" if prob <= max_loss_prob else "FAIL"))
    out = {
        "protocol_version": "v2",
        "engine_version": MC_ENGINE_VERSION,
        "path": path,
        "path_selection": path_selection,
        "decision": {
            "generator": decision_gen,
            "prob_total_return_negative": (float(prob) if prob is not None
                                           else None),
            "max_loss_prob": float(max_loss_prob),
            "mc_verdict": verdict,
            "indeterminate_reasons": list(ind),
        },
        "diagnostics": diag,
    }
    out.update(payload)
    return out
