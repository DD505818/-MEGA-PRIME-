
"""Command line for the validation lab's research tooling.

Subcommands: prepare (seal a snapshot), verify (check a sealed snapshot),
folds (purged walk-forward boundaries), evaluate (candidate return streams
with CSCV/PBO, deflated Sharpe, and empirical block bootstrap diagnostics).
Canonical home since the lab reconciliation; `backtests.lab.cli` remains as
a compatibility shim.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path

import numpy as np
import pandas as pd

from .bootstrap import empirical_block_bootstrap
from .dataset import seal_snapshot, verify_snapshot
from .metrics import candidate_metrics
from .overfit import (
    cscv_probability_of_backtest_overfitting,
    deflated_sharpe_probability,
)
from .splits import purged_walk_forward


def _write_json(path: str | Path, payload) -> None:
    Path(path).write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")


def cmd_prepare(args) -> None:
    manifest = seal_snapshot(
        args.input,
        args.output,
        source_name=args.source,
        schema_version=None if args.schema == "auto" else args.schema,
        min_rows=args.min_rows,
        allow_reorder=args.allow_reorder,
    )
    print(json.dumps(manifest, indent=2, sort_keys=True))


def cmd_verify(args) -> None:
    print(json.dumps(verify_snapshot(args.snapshot, min_rows=args.min_rows), indent=2, sort_keys=True))


def cmd_folds(args) -> None:
    manifest = verify_snapshot(args.snapshot, min_rows=args.min_rows)
    bars = pd.read_csv(Path(args.snapshot) / manifest["canonical_file"])
    folds = purged_walk_forward(
        bars["timestamp"],
        train_bars=args.train_bars,
        test_bars=args.test_bars,
        purge_bars=args.purge_bars,
        embargo_bars=args.embargo_bars,
        step_bars=args.step_bars,
        anchored=not args.rolling,
    )
    manifest_id = json.loads((Path(args.snapshot) / "manifest.json").read_text(encoding="utf-8"))["manifest_id"]
    payload = {
        "snapshot_manifest_id": manifest_id,
        "train_bars": args.train_bars,
        "test_bars": args.test_bars,
        "purge_bars": args.purge_bars,
        "embargo_bars": args.embargo_bars,
        "rolling": args.rolling,
        "folds": folds,
    }
    _write_json(args.output, payload)
    print(json.dumps(payload, indent=2, sort_keys=True))


def cmd_evaluate(args) -> None:
    frame = pd.read_csv(args.returns)
    if "timestamp" not in frame.columns:
        raise ValueError("returns file must contain timestamp plus candidate columns")
    names = [c for c in frame.columns if c != "timestamp"]
    if len(names) < 2:
        raise ValueError("returns file must contain at least two candidates")
    matrix = frame[names].to_numpy(dtype=float)
    if not np.isfinite(matrix).all():
        raise ValueError("candidate returns contain non-finite values")

    metrics = candidate_metrics(matrix, names, periods_per_year=args.periods_per_year)
    best_name = max(names, key=lambda n: metrics[n]["sharpe"])
    sharpes = [metrics[n]["sharpe"] / (args.periods_per_year ** 0.5) for n in names]
    best_raw = matrix[:, names.index(best_name)]
    payload = {
        "research_only": True,
        "selection_metric": "annualized_sharpe",
        "candidate_count": len(names),
        "best_candidate": best_name,
        "candidates": metrics,
        "deflated_sharpe": deflated_sharpe_probability(best_raw, sharpes),
        "cscv": cscv_probability_of_backtest_overfitting(matrix, slices=args.cscv_slices),
        "monte_carlo": empirical_block_bootstrap(
            best_raw,
            simulations=args.simulations,
            block_size=args.block_size,
            seed=args.seed,
            ruin_drawdown=args.ruin_drawdown,
        ),
        "note": "Validation output is research evidence, not proof of future profitability or LIVE readiness.",
    }
    _write_json(args.output, payload)
    print(json.dumps(payload, indent=2, sort_keys=True))


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="omega-validation-lab")
    sub = p.add_subparsers(dest="command", required=True)

    prepare = sub.add_parser("prepare", help="validate, canonicalize, hash, and seal a market-data snapshot")
    prepare.add_argument("--input", required=True)
    prepare.add_argument("--output", required=True)
    prepare.add_argument("--source", required=True)
    prepare.add_argument("--schema", choices=["auto", "omega-bars-v1", "omega-ticks-v1"], default="auto")
    prepare.add_argument("--min-rows", type=int, default=100)
    prepare.add_argument("--allow-reorder", action="store_true")
    prepare.set_defaults(func=cmd_prepare)

    verify = sub.add_parser("verify", help="verify a sealed snapshot before research use")
    verify.add_argument("--snapshot", required=True)
    verify.add_argument("--min-rows", type=int, default=100)
    verify.set_defaults(func=cmd_verify)

    folds = sub.add_parser("folds", help="create purged walk-forward fold boundaries")
    folds.add_argument("--snapshot", required=True)
    folds.add_argument("--output", required=True)
    folds.add_argument("--train-bars", type=int, required=True)
    folds.add_argument("--test-bars", type=int, required=True)
    folds.add_argument("--purge-bars", type=int, default=0)
    folds.add_argument("--embargo-bars", type=int, default=0)
    folds.add_argument("--step-bars", type=int)
    folds.add_argument("--rolling", action="store_true")
    folds.add_argument("--min-rows", type=int, default=100)
    folds.set_defaults(func=cmd_folds)

    evaluate = sub.add_parser("evaluate", help="evaluate candidate return streams with overfit and bootstrap diagnostics")
    evaluate.add_argument("--returns", required=True)
    evaluate.add_argument("--output", required=True)
    evaluate.add_argument("--periods-per-year", type=int, default=252)
    evaluate.add_argument("--cscv-slices", type=int, default=8)
    evaluate.add_argument("--simulations", type=int, default=1000)
    evaluate.add_argument("--block-size", type=int, default=20)
    evaluate.add_argument("--seed", type=int, default=7)
    evaluate.add_argument("--ruin-drawdown", type=float, default=0.25)
    evaluate.set_defaults(func=cmd_evaluate)
    return p


def main() -> None:
    args = build_parser().parse_args()
    args.func(args)


if __name__ == "__main__":
    main()