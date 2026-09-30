"""Overfit defenses: multiple-testing correction, backtest-length adequacy, and
parameter fragility.

1. Deflated Sharpe Ratio (Bailey & Lopez de Prado): adjusts the observed Sharpe
   for the number of configurations tried (selection bias under multiple testing).
2. Minimum backtest length: the track record must be long enough that the
   observed Sharpe is statistically distinguishable from zero.
3. Parameter sensitivity: perturb each numeric parameter +/-20% one at a time;
   the edge must survive (positive Sharpe, at least `min_frac` of base).
"""

import numpy as np
from scipy import stats as scipy_stats

from .metrics import sharpe


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
    gamma = 0.5772156649  # Euler-Mascheroni
    sr0 = np.sqrt(var_sr) * ((1 - gamma) * scipy_stats.norm.ppf(1 - 1 / k)
                             + gamma * scipy_stats.norm.ppf(1 - 1 / (k * np.e)))
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
