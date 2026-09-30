"""MIDAS admin CLI. Read-only `status` is the default safe command.

  python -m midas.cli status [--strategy X]
  python -m midas.cli promote --strategy X --report report.json [--by NAME]
  python -m midas.cli demote --strategy X --reason "..."
  python -m midas.cli record-fill --strategy X --pnl 12.5 [--notional 1000]

Exit 0 on success, 1 on failure. There is no promotion path that bypasses
report verification -- no force flag exists by design.
"""

from __future__ import annotations

import argparse
import json
import os
import sys

from . import demote as demote_mod
from .promote import promote as _promote_fn
from .registry import PromotionRegistry
from .thresholds import Thresholds


def _registry() -> PromotionRegistry:
    return PromotionRegistry(Thresholds.from_env().db_path)


def _cmd_status(args) -> int:
    reg = _registry()
    if args.strategy:
        rec = reg.get(args.strategy)
        if rec is None:
            print(json.dumps({"strategy": args.strategy, "status": "UNKNOWN (not promoted)"}))
        else:
            print(json.dumps(rec, indent=2, default=str))
    else:
        rows = reg.list()
        print(json.dumps(
            [{"strategy_id": r["strategy_id"], "status": r["status"],
              "promoted_at": r["promoted_at"], "reject_reason": r["reject_reason"]}
             for r in rows],
            indent=2,
        ))
    return 0


def _cmd_promote(args) -> int:
    reg = _registry()
    ok, msg = _promote_fn(
        reg, args.strategy, args.report, promoted_by=args.by or os.getenv("USER")
    )
    print(msg)
    return 0 if ok else 1


def _cmd_demote(args) -> int:
    reg = _registry()
    ok, msg = demote_mod.manual_demote(reg, args.strategy, args.reason)
    print(msg)
    return 0 if ok else 1


def _cmd_record_fill(args) -> int:
    reg = _registry()
    demoted, msg = demote_mod.record_fill(
        reg, args.strategy, args.pnl, notional=args.notional
    )
    print(msg)
    return 0 if not demoted else 1


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(prog="midas", description="MIDAS promotion gate admin")
    sub = p.add_subparsers(dest="cmd", required=True)

    s = sub.add_parser("status", help="show promotion state (read-only)")
    s.add_argument("--strategy", default=None)
    s.set_defaults(fn=_cmd_status)

    s = sub.add_parser("promote", help="promote a strategy from a lab PASS report")
    s.add_argument("--strategy", required=True)
    s.add_argument("--report", required=True, help="validation-lab report.json")
    s.add_argument("--by", default=None, help="operator identity recording the promotion")
    s.set_defaults(fn=_cmd_promote)

    s = sub.add_parser("demote", help="manually demote a strategy (safe direction)")
    s.add_argument("--strategy", required=True)
    s.add_argument("--reason", required=True)
    s.set_defaults(fn=_cmd_demote)

    s = sub.add_parser("record-fill", help="record a PAPER fill and re-evaluate")
    s.add_argument("--strategy", required=True)
    s.add_argument("--pnl", type=float, required=True)
    s.add_argument("--notional", type=float, default=None)
    s.set_defaults(fn=_cmd_record_fill)

    args = p.parse_args(argv)
    return args.fn(args)


if __name__ == "__main__":
    sys.exit(main())
