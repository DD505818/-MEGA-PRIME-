"""Gauntlet orchestrator. Runs every stage in doctrine order, FAIL fast, no tuning.

Stage order (all on AFTER-COST returns):
  1. data_integrity  - SHA-256 verified load of the frozen dataset
  2. costs           - net returns after fees/spread/funding; 2x/3x stress gated
  3. walkforward     - purged CV with embargo; OOS Sharpe + trade-count gates
  4. nulls           - permutation + random-timing p-values
  5. bootstrap       - stationary bootstrap CI for Sharpe
  6. montecarlo      - fat-tailed and regime-switching path simulations
  7. overfit         - parameter sensitivity, deflated Sharpe, min backtest length

A stage FAIL stops the run: later stages are marked NOT RUN. There are no
tuning hooks -- the config is fixed at construction and the signal factory
receives only the base params (sensitivity perturbations are prescribed).
"""

import json
import os
import subprocess
from dataclasses import asdict, dataclass, field

import numpy as np
import pandas as pd
from scipy import stats as scipy_stats

from . import bootstrap, costs, data, montecarlo, nulls, overfit, walkforward
from .metrics import annualized_return, max_drawdown, sharpe, strategy_returns, total_return


@dataclass
class LabConfig:
    # costs (bps)
    fee_bps: float = 10.0
    spread_bps: float = 5.0
    funding_annual_bps: float = 0.0
    periods_per_year: int = 24 * 365
    # walk-forward
    n_folds: int = 5
    embargo_pct: float = 0.01
    # stochastic stages
    n_perm: int = 2000
    n_boot: int = 2000
    mean_block: int = 24 * 7
    n_mc_paths: int = 2000
    seed: int = 42
    # pass thresholds
    min_net_sharpe: float = 0.0
    cost_stress_2x_min_sharpe: float = 0.0  # strict gate: 2x net Sharpe > this
    cost_stress_3x_min_sharpe: float = -0.5  # floor: 3x net Sharpe >= this
    min_oos_sharpe: float = 0.5
    min_positive_fold_frac: float = 0.6
    min_oos_trades: int = 100
    min_oos_trades_per_fold: int = 10
    max_pvalue: float = 0.05
    min_dsr: float = 0.95
    textbook_dsr_gate_a_floor: float = 0.90  # reported Gate-A floor, not a lab verdict gate
    sensitivity_min_frac: float = 0.5
    max_mc_loss_prob: float = 0.20
    # --- validation protocol v2 lineage ---
    # Caller-supplied per hypothesis; the lab fills the rest (data hash,
    # code commit, cost/MC engine versions) so every report is auditable.
    validation_protocol_version: str = "v2"  # "v1" = campaigns #1-2 legacy
    rng_seed: int | None = None              # defaults to `seed` when unset
    strategy_spec_hash: str = ""
    preregistration_id: str = ""
    hypothesis_number: int = 0

    def to_dict(self):
        return asdict(self)


STAGE_ORDER = ["data_integrity", "costs", "walkforward", "nulls",
               "bootstrap", "montecarlo", "overfit"]


def _stage(name, status, metrics, threshold=None):
    return {"status": status, "metrics": metrics,
            "threshold": threshold}


def run_gauntlet(csv_path, signal_factory, base_params, config=None,
                 candidate="candidate"):
    """Run the full gauntlet. Returns the report dict (JSON-serializable)."""
    cfg = config or LabConfig()
    ppy = cfg.periods_per_year
    stages = {}
    failed_stage = None

    def fail_fast(name, metrics, threshold):
        nonlocal failed_stage
        stages[name] = _stage(name, "FAIL", metrics, threshold)
        failed_stage = failed_stage or name
        for later in STAGE_ORDER[STAGE_ORDER.index(name) + 1:]:
            stages[later] = _stage(later, "NOT RUN", {}, None)

    # ---- 1. data integrity ----
    try:
        manifest = data.verify(csv_path)
        df = data.load_csv(csv_path, verify_hash=False)
        desc = data.describe(df, manifest, csv_path)
    except data.IntegrityError as e:
        stages["data_integrity"] = _stage("data_integrity", "FAIL", {"error": str(e)}, None)
        for later in STAGE_ORDER[1:]:
            stages[later] = _stage(later, "NOT RUN", {}, None)
        return _report(candidate, cfg, None, stages, "FAIL", "data_integrity")
    stages["data_integrity"] = _stage("data_integrity", "PASS", desc, None)

    # ---- signal + gross returns (causal) ----
    positions = pd.Series(signal_factory(dict(base_params)), index=df.index
                          ).fillna(0.0).clip(-1, 1)
    asset_rets = df["close"].pct_change().fillna(0.0).to_numpy()
    gross = strategy_returns(df, positions).to_numpy()
    pos_arr = positions.to_numpy()

    # ---- 2. costs ----
    c = costs.apply_costs(gross, pos_arr, cfg.fee_bps, cfg.spread_bps,
                          cfg.funding_annual_bps, ppy)
    net = np.asarray(c["net_returns"], dtype=float)
    c["cost_stress"] = costs.cost_stress(
        gross, pos_arr, cfg.fee_bps, cfg.spread_bps,
        cfg.funding_annual_bps, ppy)
    c["net_max_drawdown"] = max_drawdown(net)
    c["net_ann_return"] = annualized_return(net, ppy)
    stress_2x = c["cost_stress"]["2x"]["net_sharpe"]
    stress_3x = c["cost_stress"]["3x"]["net_sharpe"]
    thr = {"min_net_sharpe": cfg.min_net_sharpe,
           "cost_stress_2x_min_sharpe": cfg.cost_stress_2x_min_sharpe,
           "cost_stress_2x_operator": ">",
           "cost_stress_3x_min_sharpe": cfg.cost_stress_3x_min_sharpe,
           "cost_stress_3x_operator": ">="}
    if (c["net_sharpe"] > cfg.min_net_sharpe
            and stress_2x > cfg.cost_stress_2x_min_sharpe
            and stress_3x >= cfg.cost_stress_3x_min_sharpe):
        stages["costs"] = _stage("costs", "PASS", _jsonable(c), thr)
    else:
        fail_fast("costs", _jsonable(c), thr)
        return _report(candidate, cfg, desc, stages, "FAIL", failed_stage)

    # ---- 3. walk-forward (purged CV + embargo) ----
    splits = walkforward.purged_cv_splits(len(net), cfg.n_folds, cfg.embargo_pct)
    wf = walkforward.summarize(walkforward.fold_metrics(net, splits, ppy,
                                                        positions=pos_arr))
    oos_trades = int(wf.get("oos_trades", 0))
    oos_min_fold = int(wf.get("oos_trades_min_per_fold", 0))
    trade_gate_passed = (oos_trades >= cfg.min_oos_trades
                         and oos_min_fold >= cfg.min_oos_trades_per_fold)
    wf["oos_trade_gate"] = {
        "passed": bool(trade_gate_passed),
        "reason": None if trade_gate_passed else (
            f"OOS trade count gate failed: total {oos_trades} < "
            f"{cfg.min_oos_trades} or min per fold {oos_min_fold} < "
            f"{cfg.min_oos_trades_per_fold}"
        ),
    }
    thr = {"min_oos_sharpe": cfg.min_oos_sharpe,
           "min_positive_fold_frac": cfg.min_positive_fold_frac,
           "min_oos_trades": cfg.min_oos_trades,
           "min_oos_trades_per_fold": cfg.min_oos_trades_per_fold}
    if (wf["mean_test_sharpe"] >= cfg.min_oos_sharpe
            and wf["positive_fold_frac"] >= cfg.min_positive_fold_frac
            and trade_gate_passed):
        stages["walkforward"] = _stage("walkforward", "PASS", _jsonable(wf), thr)
    else:
        fail_fast("walkforward", _jsonable(wf), thr)
        return _report(candidate, cfg, desc, stages, "FAIL", failed_stage)

    # ---- 4. nulls ----
    perm = nulls.permutation_test(net, cfg.n_perm, cfg.seed, ppy)
    rt = nulls.random_timing_test(asset_rets, pos_arr, cfg.n_perm, cfg.seed + 1, ppy)
    bh = nulls.buy_and_hold_comparison(asset_rets, net, cfg.n_perm, cfg.seed + 2, ppy)
    nm = {"permutation": perm, "random_timing": rt, "buy_and_hold": bh}
    thr = {"max_pvalue": cfg.max_pvalue}
    if perm["p_value"] <= cfg.max_pvalue and rt["p_value"] <= cfg.max_pvalue:
        stages["nulls"] = _stage("nulls", "PASS", _jsonable(nm), thr)
    else:
        fail_fast("nulls", _jsonable(nm), thr)
        return _report(candidate, cfg, desc, stages, "FAIL", failed_stage)

    # ---- 5. bootstrap ----
    bs = bootstrap.stationary_bootstrap(net, cfg.n_boot, cfg.mean_block,
                                        cfg.seed + 3, ppy)
    thr = {"sharpe_ci_low_gt": 0.0}
    if bs["sharpe_ci"][0] > 0:
        stages["bootstrap"] = _stage("bootstrap", "PASS", _jsonable(bs), thr)
    else:
        fail_fast("bootstrap", _jsonable(bs), thr)
        return _report(candidate, cfg, desc, stages, "FAIL", failed_stage)

    # ---- 6. monte carlo (protocol v2: two-path engine) ----
    mc = montecarlo.run_v2(net, cfg.n_mc_paths, cfg.seed + 4, ppy,
                            max_loss_prob=cfg.max_mc_loss_prob)
    dec = mc["decision"]
    thr = {"max_mc_loss_prob": cfg.max_mc_loss_prob,
           "mc_verdict_must_be": "PASS"}
    if dec["mc_verdict"] == "PASS":
        stages["montecarlo"] = _stage("montecarlo", "PASS", _jsonable(mc), thr)
    else:
        # FAIL and INDETERMINATE both fail the stage: INDETERMINATE means
        # the gate could not measure the strategy, so it cannot advance.
        fail_fast("montecarlo", _jsonable(mc), thr)
        return _report(candidate, cfg, desc, stages, "FAIL", failed_stage)

    # ---- 7. overfit ----
    sens = overfit.parameter_sensitivity(signal_factory, base_params, df, ppy,
                                         min_frac=cfg.sensitivity_min_frac)
    skew = float(scipy_stats.skew(net))
    kurt = float(scipy_stats.kurtosis(net) + 3.0)
    dsr, sr0 = overfit.deflated_sharpe_ratio(
        c["net_sharpe"], sens["trial_sharpes"], len(net), skew, kurt)
    textbook_dsr, textbook_sr0 = overfit.textbook_deflated_sharpe_ratio(
        c["net_sharpe"], sens["trial_sharpes"], len(net), ppy, skew, kurt)
    req_len = overfit.min_backtest_length(max(c["net_sharpe"], 1e-9), skew, kurt,
                                          periods_per_year=ppy)
    om = {"sensitivity": {k: v for k, v in sens.items() if k != "trial_sharpes"},
          "trial_sharpes": sens["trial_sharpes"],
          "deflated_sharpe": dsr, "expected_sr_under_null": sr0,
          "textbook_dsr": textbook_dsr,
          "textbook_expected_sr_under_null": textbook_sr0,
          "textbook_dsr_scope": "candidate-local sensitivity trials; "
                                "campaign-wide trial count is not included "
                                "until the campaign log exists",
          "n_obs": len(net), "min_backtest_length": req_len,
          "skew": skew, "kurtosis": kurt}
    thr = {"min_dsr": cfg.min_dsr, "sensitivity_must_survive": True,
           "n_obs_ge_min_length": True}
    if dsr >= cfg.min_dsr and sens["survived"] and len(net) >= req_len:
        stages["overfit"] = _stage("overfit", "PASS", _jsonable(om), thr)
    else:
        fail_fast("overfit", _jsonable(om), thr)
        return _report(candidate, cfg, desc, stages, "FAIL", failed_stage)

    return _report(candidate, cfg, desc, stages, "PASS", None)


def _jsonable(obj):
    if isinstance(obj, dict):
        return {k: _jsonable(v) for k, v in obj.items()}
    if isinstance(obj, (list, tuple)):
        return [_jsonable(v) for v in obj]
    if isinstance(obj, (np.floating, np.integer)):
        return obj.item()
    if isinstance(obj, np.ndarray):
        return obj.tolist()
    if isinstance(obj, (pd.Series, pd.DataFrame)):
        return obj.to_dict()
    return obj


def _code_commit():
    """Best-effort git commit of the lab source; UNKNOWN when unavailable."""
    try:
        out = subprocess.run(
            ["git", "-C", os.path.dirname(os.path.abspath(__file__)),
             "rev-parse", "HEAD"],
            capture_output=True, text=True, timeout=10)
        commit = out.stdout.strip()
        return commit if commit else "UNKNOWN"
    except Exception:
        return "UNKNOWN"


def _lineage(cfg, desc):
    data_hash = desc.get("sha256") if isinstance(desc, dict) else None
    return {
        "strategy_spec_hash": cfg.strategy_spec_hash or "UNKNOWN",
        "data_snapshot_hash": data_hash or "UNKNOWN",
        "code_commit": _code_commit(),
        "validation_protocol_version": cfg.validation_protocol_version,
        "rng_seed": cfg.rng_seed if cfg.rng_seed is not None else cfg.seed,
        "cost_model_version": costs.COST_MODEL_VERSION,
        "mc_engine_version": montecarlo.MC_ENGINE_VERSION,
        "preregistration_id": cfg.preregistration_id or "UNKNOWN",
        "hypothesis_number": cfg.hypothesis_number,
    }


def _report(candidate, cfg, desc, stages, verdict, failed_stage):
    return {
        "candidate": candidate,
        "verdict": verdict,
        "failed_stage": failed_stage,
        "config": cfg.to_dict(),
        "data": desc,
        "stages": stages,
        "lineage": _lineage(cfg, desc),
        "doctrine": "EDGE NOT PROVEN unless verdict == PASS on all stages; "
                    "PASS here qualifies for PAPER only, never LIVE.",
    }


def write_report(report, path):
    with open(path, "w") as f:
        json.dump(report, f, indent=2)
    return path
