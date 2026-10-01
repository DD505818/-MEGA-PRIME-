import hashlib
import json

import numpy as np
import pandas as pd

from validation_lab import data
from validation_lab.lab import LabConfig, run_gauntlet


def _make_csv(tmp_path, name, n=3000, seed=0, drift=0.0002):
    rng = np.random.default_rng(seed)
    px = 100 * np.cumprod(1 + drift + rng.normal(0, 0.008, size=n))
    idx = pd.date_range("2023-01-01", periods=n, freq="h", tz="UTC")
    df = pd.DataFrame({
        "ts": (idx.view("int64") // 10 ** 9).astype(int),
        "iso8601": idx.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "open": px, "high": px * 1.001, "low": px * 0.999,
        "close": px, "volume": 1.0,
    })
    p = tmp_path / name
    df.to_csv(p, index=False)
    h = hashlib.sha256(p.read_bytes()).hexdigest()
    man = {"venue": "t", "pair": "T",
           "files": {"1h": {"file": name, "sha256": h}}}
    (tmp_path / "MANIFEST.json").write_text(json.dumps(man))
    return str(p)


def _write_close_csv(tmp_path, name, close):
    close = np.asarray(close, dtype=float)
    idx = pd.date_range("2023-01-01", periods=len(close), freq="h", tz="UTC")
    df = pd.DataFrame({
        "ts": (idx.view("int64") // 10 ** 9).astype(int),
        "iso8601": idx.strftime("%Y-%m-%dT%H:%M:%SZ"),
        "open": close, "high": close * 1.001, "low": close * 0.999,
        "close": close, "volume": 1.0,
    })
    p = tmp_path / name
    df.to_csv(p, index=False)
    h = hashlib.sha256(p.read_bytes()).hexdigest()
    man = {"venue": "synthetic", "pair": "SYN/USD",
           "files": {"1h": {"file": name, "sha256": h}}}
    (tmp_path / "MANIFEST.json").write_text(json.dumps(man))
    return str(p)


def _cfg(**kw):
    base = dict(n_folds=3, n_perm=100, n_boot=100, n_mc_paths=50,
                mean_block=12, seed=0)
    base.update(kw)
    return LabConfig(**base)


def test_fail_fast_on_costs(tmp_path):
    # constant-zero signal: gross Sharpe = 0 -> costs stage FAILs fast
    p = _make_csv(tmp_path, "t_1h.csv")

    def factory(params):
        df = data.load_csv(p)
        return pd.Series(0.0, index=df.index)

    rep = run_gauntlet(p, factory, {}, _cfg(), candidate="zero")
    assert rep["verdict"] == "FAIL"
    assert rep["failed_stage"] == "costs"
    assert rep["stages"]["walkforward"]["status"] == "NOT RUN"
    assert rep["stages"]["data_integrity"]["status"] == "PASS"


def test_cost_stress_gate_fails_at_costs(tmp_path):
    # Perfectly timed alternating returns: +0.4% gross per bar, but the
    # signal flips every bar. 1x costs leave ~+0.1%/bar; 2x costs erase it.
    n = 600
    rets = np.zeros(n)
    rets[1:] = np.where(np.arange(1, n) % 2 == 1, 0.004, -0.004)
    close = 100 * np.cumprod(1 + rets)
    p = _write_close_csv(tmp_path, "stress_1h.csv", close)
    df = data.load_csv(p)

    def factory(params):
        r = df["close"].pct_change().fillna(0.0)
        pos = -np.sign(r)
        pos.iloc[0] = 1.0
        return pd.Series(pos, index=df.index)

    rep = run_gauntlet(p, factory, {}, _cfg(), candidate="stress-fragile")
    costs = rep["stages"]["costs"]
    assert rep["verdict"] == "FAIL"
    assert rep["failed_stage"] == "costs"
    assert costs["metrics"]["net_sharpe"] > 0
    assert costs["metrics"]["cost_stress"]["2x"]["net_sharpe"] <= 0


def test_synthetic_ar_momentum_passes_all_stages(tmp_path):
    # Harness fixture (not campaign evidence): AR(1) returns with a causal
    # return-momentum signal. Strong enough to exercise every new gate.
    n = 6000
    rng = np.random.default_rng(11)
    asset = np.empty(n)
    asset[0] = rng.normal(0, 0.0075)
    for i in range(1, n):
        asset[i] = 0.00012 + 0.52 * asset[i - 1] + rng.normal(0, 0.0075)
    close = 100 * np.cumprod(1 + asset)
    p = _write_close_csv(tmp_path, "synthetic_1h.csv", close)
    df = data.load_csv(p)
    rets = df["close"].pct_change().fillna(0.0)

    def factory(params):
        fast = int(params["fast"])
        slow = int(params["slow"])
        score = rets.rolling(fast).mean() - rets.rolling(slow).mean()
        pos = pd.Series(np.sign(score).fillna(0.0), index=df.index)
        pos.iloc[:slow] = 0.0
        return pos

    cfg = _cfg(n_folds=5, n_perm=200, n_boot=200, n_mc_paths=100,
               mean_block=24, seed=2)
    rep = run_gauntlet(p, factory, {"fast": 3, "slow": 16}, cfg,
                       candidate="synthetic-ar-momentum")
    assert rep["verdict"] == "PASS", rep["failed_stage"]
    assert all(st["status"] == "PASS" for st in rep["stages"].values())
    wf = rep["stages"]["walkforward"]["metrics"]
    assert wf["oos_trades"] >= 100
    assert wf["oos_trades_min_per_fold"] >= 10
    om = rep["stages"]["overfit"]["metrics"]
    assert om["textbook_dsr"] >= 0.90


def test_report_structure(tmp_path):
    p = _make_csv(tmp_path, "t_1h.csv")

    def factory(params):
        df = data.load_csv(p)
        return pd.Series(0.0, index=df.index)

    rep = run_gauntlet(p, factory, {}, _cfg(), candidate="zero")
    assert set(rep["stages"]) == {"data_integrity", "costs", "walkforward",
                                  "nulls", "bootstrap", "montecarlo", "overfit"}
    assert rep["data"]["sha256"]
    assert "doctrine" in rep
    # JSON-serializable
    json.dumps(rep)


def test_integrity_failure_kills_run(tmp_path):
    p = _make_csv(tmp_path, "t_1h.csv")
    with open(p, "ab") as f:
        f.write(b"tamper")

    def factory(params):
        return pd.Series(1.0, index=pd.RangeIndex(10))

    rep = run_gauntlet(p, factory, {}, _cfg(), candidate="x")
    assert rep["verdict"] == "FAIL"
    assert rep["failed_stage"] == "data_integrity"
