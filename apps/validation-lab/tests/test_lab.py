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
