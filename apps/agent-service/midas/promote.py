"""Promotion logic: lab PASS report -> PROMOTED. No overrides, no force flag.

A strategy is promoted only if ALL of the following hold:
  1. the report is a dict with verdict == "PASS" and failed_stage is None
  2. every gauntlet stage in STAGE_ORDER is present with status == "PASS"
     (explicitly including the after-cost `costs` stage)
  3. the report's config bar is at least as strict as MIDAS's bar --
     a report generated with weaker thresholds is rejected
  4. the report's data SHA-256 matches a hash in a known frozen dataset
     manifest (kraken/coinbase MANIFEST.json under MIDAS_DATASET_DIR)
  5. for a DEMOTED strategy, the report is FRESH: its SHA-256 was never
     consumed by a prior promotion of that strategy

Anything else -> REJECTED with the reason recorded. Fail closed throughout.
"""

from __future__ import annotations

import glob
import hashlib
import json
import os
from typing import Callable

from .registry import PromotionRegistry, DEMOTED, PROMOTED, REJECTED
from .thresholds import Thresholds

STAGE_ORDER = (
    "data_integrity",
    "costs",
    "walkforward",
    "nulls",
    "bootstrap",
    "montecarlo",
    "overfit",
)

def _bar() -> list[tuple[str, str, Callable[[float, float], bool], str]]:
    """(report config key, Thresholds attr, comparator, operator symbol).

    The report's own thresholds must satisfy the comparator against the
    MIDAS bar: ">=" means the report bar must be at least as strict
    (higher), "<=" means at most (lower)."""

    return [
        ("min_net_sharpe", "min_net_sharpe", lambda r, b: r >= b, ">="),
        ("min_oos_sharpe", "min_oos_sharpe", lambda r, b: r >= b, ">="),
        ("min_positive_fold_frac", "min_positive_fold_frac", lambda r, b: r >= b, ">="),
        ("max_pvalue", "max_pvalue", lambda r, b: r <= b, "<="),
        ("min_dsr", "min_dsr", lambda r, b: r >= b, ">="),
        ("sensitivity_min_frac", "sensitivity_min_frac", lambda r, b: r >= b, ">="),
        ("max_mc_loss_prob", "max_mc_loss_prob", lambda r, b: r <= b, "<="),
    ]


def known_data_hashes(dataset_dir: str | None = None) -> set[str]:
    """All frozen CSV SHA-256 hashes recorded in the venue manifests."""
    from .thresholds import Thresholds as _T

    root = dataset_dir or _T.from_env().dataset_dir
    hashes: set[str] = set()
    for manifest in glob.glob(os.path.join(root, "*", "MANIFEST.json")):
        try:
            with open(manifest) as f:
                m = json.load(f)
            files = m.get("files", {})
            for _gran, entry in files.items():
                h = entry.get("sha256") if isinstance(entry, dict) else None
                if h:
                    hashes.add(str(h).lower())
        except Exception:
            continue  # an unreadable manifest contributes nothing; fail closed later
    return hashes


def _report_sha256(raw: bytes) -> str:
    return hashlib.sha256(raw).hexdigest()


def _snapshot_metrics(rep: dict) -> dict:
    stages = rep.get("stages", {})

    def m(stage: str, key: str):
        try:
            return stages[stage]["metrics"][key]
        except Exception:
            return None

    return {
        "net_sharpe": m("costs", "net_sharpe"),
        "mean_oos_sharpe": m("walkforward", "mean_test_sharpe"),
        "permutation_p": (m("nulls", "permutation") or {}).get("p_value")
        if isinstance(m("nulls", "permutation"), dict)
        else None,
        "random_timing_p": (m("nulls", "random_timing") or {}).get("p_value")
        if isinstance(m("nulls", "random_timing"), dict)
        else None,
        "deflated_sharpe": m("overfit", "deflated_sharpe"),
        "mc_loss_prob": (m("montecarlo", "regime_switching") or {}).get(
            "prob_total_return_negative"
        )
        if isinstance(m("montecarlo", "regime_switching"), dict)
        else None,
        "data": rep.get("data", {}),
    }


def promote(
    registry: PromotionRegistry,
    strategy_id: str,
    report,
    promoted_by: str | None = None,
    thresholds: Thresholds | None = None,
    dataset_dir: str | None = None,
) -> tuple[bool, str]:
    """Evaluate a validation-lab report. Returns (ok, message)."""
    t = thresholds or Thresholds.from_env()

    def reject(reason: str) -> tuple[bool, str]:
        registry.set_rejected(strategy_id, reason)
        return False, reason

    try:
        if isinstance(report, (str, bytes, os.PathLike)):
            with open(report, "rb") as f:
                raw = f.read()
            rep = json.loads(raw.decode("utf-8"))
        elif isinstance(report, dict):
            rep = report
            raw = json.dumps(rep, sort_keys=True).encode("utf-8")
        else:
            return reject("report must be a JSON file path or dict")
    except Exception as exc:
        return reject(f"unreadable report: {exc}")

    if not isinstance(rep, dict):
        return reject("report is not a JSON object")

    report_hash = _report_sha256(raw)

    # Capture prior state BEFORE marking VALIDATING (which overwrites status).
    prev_rec = registry.get(strategy_id)
    prev_status = prev_rec["status"] if prev_rec else None
    prev_report_hash = prev_rec.get("report_sha256") if prev_rec else None

    # Freshness fast-path, before any state change: a report hash is consumed
    # at most once per strategy, ever (current promotion + every demotion
    # entry). A stale report is rejected without disturbing current state,
    # so it can never sneak back in through a REJECTED round-trip, and a
    # DEMOTED strategy keeps its PAPER-failure state.
    if prev_status == PROMOTED and prev_report_hash == report_hash:
        return True, f"{strategy_id} already PROMOTED on this report (idempotent)"
    if report_hash in registry.used_report_hashes(strategy_id):
        reason = (
            "report already consumed by a prior promotion of this strategy; "
            "a fresh PASS report is required"
        )
        registry.record_reject_reason(strategy_id, reason)
        return False, reason

    registry.set_validating(strategy_id)

    # 1. verdict
    if rep.get("verdict") != "PASS":
        return reject(
            f"verdict is {rep.get('verdict')!r}, not PASS "
            f"(failed_stage={rep.get('failed_stage')!r})"
        )
    if rep.get("failed_stage") is not None:
        return reject(f"failed_stage={rep.get('failed_stage')!r} despite verdict PASS")

    # 2. every stage PASS, including after-cost `costs`
    stages = rep.get("stages")
    if not isinstance(stages, dict):
        return reject("report has no stages object")
    for name in STAGE_ORDER:
        st = stages.get(name)
        if not isinstance(st, dict) or st.get("status") != "PASS":
            got = st.get("status") if isinstance(st, dict) else None
            return reject(f"stage {name!r} status is {got!r}, not PASS")

    # 3. report bar at least as strict as MIDAS bar
    cfg = rep.get("config")
    if not isinstance(cfg, dict):
        return reject("report has no config object")
    for key, attr, cmp, op in _bar():
        if key not in cfg:
            return reject(f"report config missing threshold {key!r}")
        try:
            rv, bv = float(cfg[key]), float(getattr(t, attr))
        except (TypeError, ValueError):
            return reject(f"report config threshold {key!r} is not numeric")
        if not cmp(rv, bv):
            return reject(
                f"report generated with weaker bar: config.{key}={rv} "
                f"does not satisfy {op} MIDAS bar {bv}"
            )

    # 4. data hash matches a frozen dataset manifest
    data = rep.get("data") or {}
    data_hash = str(data.get("sha256", "")).lower()
    known = known_data_hashes(dataset_dir or t.dataset_dir)
    if not known:
        return reject("no frozen dataset manifests readable; cannot verify data provenance")
    if not data_hash or data_hash not in known:
        return reject(
            f"report data sha256 {data_hash[:16] or '?'}... not found in any "
            f"frozen dataset manifest"
        )

    # (freshness was verified in the fast-path above, before VALIDATING)

    registry.set_promoted(
        strategy_id,
        verdict="PASS",
        report_sha256=report_hash,
        data_sha256=data_hash,
        metrics=_snapshot_metrics(rep),
        promoted_by=promoted_by,
    )
    return True, f"{strategy_id} PROMOTED on lab report {report_hash[:12]}..."
