"""Overfit defenses: multiple-testing correction, backtest-length adequacy, and
parameter fragility.

1. Deflated Sharpe Ratio (Bailey & Lopez de Prado): adjusts the observed Sharpe
   for the number of configurations tried (selection bias under multiple testing).
2. Minimum backtest length: the track record must be long enough that the
   observed Sharpe is statistically distinguishable from zero.
3. Parameter sensitivity: perturb each numeric parameter +/-20% one at a time;
   the edge must survive (positive Sharpe, at least `min_frac` of base).
4. CSCV Probability of Backtest Overfitting (Bailey et al.): combinatorially
   symmetric cross-validation over candidate return streams.

This module is the canonical home for the multiple-testing statistics.
`probabilistic_sharpe_probability`, `deflated_sharpe_probability`, and
`cscv_probability_of_backtest_overfitting` were consolidated here from the
former `backtests.lab.statistics` module; they share the null-benchmark
helper with `deflated_sharpe_ratio`.
"""

import itertools
import math

import numpy as np
from scipy import stats as scipy_stats

from .metrics import _clean, sharpe

_EULER_GAMMA = 0.5772156649015329


def _null_benchmark(var_sr: float, n_trials: int) -> float:
    """Expected maximum Sharpe under the null, given the trial-Sharpe variance.

    Bailey & Lopez de Prado: E[max] ~= sqrt(V) * [(1-gamma) * Phi^-1(1 - 1/K)
    + gamma * Phi^-1(1 - 1/(K*e))] for K independent trials.
    """
    return math.sqrt(var_sr) * (
        (1 - _EULER_GAMMA) * scipy_stats.norm.ppf(1 - 1 / n_trials)
        + _EULER_GAMMA * scipy_stats.norm.ppf(1 - 1 / (n_trials * math.e))
    )


def expected_max_sharpe_under_null(candidate_sharpes) -> dict:
    """Expected best-of-K Sharpe if all candidates were pure noise.

    Returns {"candidate_trials": float, "expected_max_sharpe_under_null": float}.
    With <= 1 trial there is no selection bias, so the benchmark is 0.0.
    """
    finite = np.asarray([x for x in candidate_sharpes if math.isfinite(x)], dtype=float)
    trials = int(len(finite))
    if trials <= 1:
        benchmark = 0.0
    else:
        benchmark = _null_benchmark(float(np.var(finite, ddof=1)), trials)
    return {
        "candidate_trials": float(trials),
        "expected_max_sharpe_under_null": float(benchmark),
    }


def deflated_sharpe_ratio(observed_sr, trial_sharpes, n_obs, skew=0.0, kurt=3.0):
    """Deflated Sharpe Ratio (Bailey & Lopez de Prado 2014) with non-Gaussian
    correction.

    observed_sr: annualized Sharpe of the selected configuration.
    trial_sharpes: annualized Sharpes of ALL tried configurations (estimates
      the null variance V -- the selection-bias correction).
    n_obs: number of return observations. skew/kurt: of observed returns.
    Returns (dsr, expected_sr_under_null). DSR is P(true SR > 0).
    """
    ts = np.asarray(trial_sharpes, dtype=float)
    ts = ts[np.isfinite(ts)]
    if ts.size < 2:
        raise ValueError("need >= 2 trial Sharpes to estimate null variance")
    k = ts.size
    var_sr = max(float(np.var(ts, ddof=1)), 1e-12)
    sr0 = _null_benchmark(var_sr, k)
    denom = np.sqrt(max(1e-12, 1 - skew * observed_sr + (kurt / 4.0) * observed_sr ** 2))
    dsr = float(scipy_stats.norm.cdf((observed_sr - sr0) * np.sqrt(n_obs - 1) / denom))
    return dsr, float(sr0)


def min_backtest_length(target_sr_annual, skew=0.0, kurt=3.0, alpha=0.05,
                        periods_per_year=24 * 365):
    """Minimum number of observations so target_sr is significant at level alpha.

    Bailey et al.: minTRL = 1 + (1 - skew*SR + kurt/4*SR^2) * (z_alpha / SR)^2,
    with SR in per-period units.
    """
    sr_per = target_sr_annual / np.sqrt(periods_per_year)
    if sr_per <= 0:
        raise ValueError("target_sr_annual must be positive")
    z = scipy_stats.norm.ppf(1 - alpha)
    factor = 1 - skew * sr_per + (kurt / 4.0) * sr_per ** 2
    return int(1 + factor * (z / sr_per) ** 2)


def probabilistic_sharpe_probability(values, *, benchmark_sharpe: float = 0.0) -> float:
    """Probabilistic Sharpe Ratio: P(true SR > benchmark) for one return stream.

    Per-period Sharpe units; skew/kurtosis are the stream's own moments.
    Strict input validation via `_clean`.
    """
    r = _clean(values)
    n = len(r)
    sr = sharpe(r, periods_per_year=1)
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
    return float(scipy_stats.norm.cdf(z))


def deflated_sharpe_probability(best_returns, candidate_sharpes) -> dict:
    """Deflated Sharpe probability for the best of K candidate streams.

    candidate_sharpes: per-period Sharpes of ALL tried candidates. The
    benchmark is the expected maximum under the null (selection bias);
    the probability is the PSR of `best_returns` against that benchmark.
    """
    bench = expected_max_sharpe_under_null(candidate_sharpes)
    return {
        **bench,
        "deflated_sharpe_probability": probabilistic_sharpe_probability(
            best_returns, benchmark_sharpe=bench["expected_max_sharpe_under_null"]
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
) -> dict:
    """CSCV Probability of Backtest Overfitting (Bailey et al.).

    Splits the [time, candidates] matrix into `slices` blocks, evaluates
    every half/half train/test combination, and measures how often the
    in-sample winner ranks below the median out of sample.
    """
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


def parameter_sensitivity(signal_factory, base_params, df,
                         periods_per_year=24 * 365, perturb=0.20, min_frac=0.5):
    """Perturb each numeric param +/-perturb one at a time; edge must survive.

    signal_factory(params) -> Series of positions in {-1,0,1} aligned to df.
    Returns per-param Sharpes and a survival flag.
    """
    from .metrics import strategy_returns

    def sr_of(pos):
        return sharpe(strategy_returns(df, pos).to_numpy(), periods_per_year)

    base_pos = signal_factory(dict(base_params))
    base_sr = sr_of(base_pos)
    results = {}
    survived = True
    for name, val in base_params.items():
        if not isinstance(val, (int, float)) or isinstance(val, bool):
            continue
        for direction in (+1, -1):
            new_val = val * (1 + direction * perturb)
            if isinstance(val, int):
                new_val = max(1, int(round(new_val)))
            p = dict(base_params)
            p[name] = new_val
            s = sr_of(signal_factory(p))
            key = f"{name}{'+' if direction > 0 else '-'}{int(perturb * 100)}%"
            results[key] = {"param": name, "value": new_val, "sharpe": s}
            if not (s > 0 and s >= min_frac * base_sr):
                survived = False
    return {"base_sharpe": base_sr, "perturb": perturb, "min_frac": min_frac,
            "survived": survived, "trials": results,
            "trial_sharpes": [base_sr] + [t["sharpe"] for t in results.values()]}
