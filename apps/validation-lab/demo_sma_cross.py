#!/usr/bin/env python3
"""Demo: run a trivial SMA-cross candidate through the gauntlet against the
frozen Coinbase 1h dataset. Expected to FAIL -- the point is the harness and
the report format, not the signal."""

import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import pandas as pd

from validation_lab import LabConfig, run_gauntlet, write_report
from validation_lab.data import load_csv

DATA = os.path.expanduser(
    "~/workspace/datasets/omega-prime/coinbase/coinbase_BTC-USD_1h.csv")


def sma_cross_factory(params):
    """Causal SMA cross: +1 when fast SMA > slow SMA, else -1."""
    df = load_csv(DATA)  # hash-verified inside run_gauntlet too; here for index
    fast = df["close"].rolling(params["fast"]).mean()
    slow = df["close"].rolling(params["slow"]).mean()
    pos = pd.Series(0.0, index=df.index)
    pos[fast > slow] = 1.0
    pos[fast <= slow] = -1.0
    return pos


def main():
    config = LabConfig(
        fee_bps=10.0,
        spread_bps=5.0,
        funding_annual_bps=0.0,
        n_folds=5,
        n_perm=1000,
        n_boot=1000,
        n_mc_paths=200,
        seed=42,
    )
    print("running gauntlet: sma-cross-20-50 on frozen coinbase 1h ...",
          flush=True)
    report = run_gauntlet(DATA, sma_cross_factory, {"fast": 20, "slow": 50},
                          config, candidate="sma-cross-20-50")
    out = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                       "demo_report.json")
    write_report(report, out)
    print(f"verdict: {report['verdict']}  failed_stage: {report['failed_stage']}")
    for name in ("data_integrity", "costs", "walkforward", "nulls",
                 "bootstrap", "montecarlo", "overfit"):
        st = report["stages"][name]
        print(f"  {name:15s} {st['status']}")
    c = report["stages"]["costs"]["metrics"]
    print(f"  net_sharpe={c['net_sharpe']:.3f} gross_sharpe={c['gross_sharpe']:.3f} "
          f"net_total={c['net_total']:.3f} n_trades={c['n_trades']}")
    print(f"report -> {out}")


if __name__ == "__main__":
    main()
