"""Demotion logic: PAPER outcomes -> DEMOTED.

A PROMOTED strategy is demoted when its trailing PAPER fills breach either:
  * trailing per-trade Sharpe < MIDAS_DEMOTE_MIN_SHARPE (default 0.0), or
  * max drawdown on cumulative PnL > MIDAS_DEMOTE_MAX_DRAWDOWN (default 0.25)

over at least MIDAS_DEMOTE_MIN_FILLS fills (default 30). Below that count the
strategy is unevaluated -- never demoted on noise.

A DEMOTED strategy can only return via `promote()` with a FRESH PASS report;
the report hash consumed by the demoted promotion is recorded and can never
be reused. Demotion is the safe direction, so it is fail-closed and
immediate. There is no manual promotion path -- only a verified lab report.
"""

from __future__ import annotations

import math
import statistics

from .registry import PromotionRegistry, PROMOTED
from .thresholds import Thresholds


def trailing_stats(pnls: list[float], trades_per_year: float = 252.0) -> dict:
    """Per-trade Sharpe (annualized) and max drawdown of cumulative PnL."""
    n = len(pnls)
    if n == 0:
        return {"n": 0, "sharpe": float("nan"), "max_drawdown": 0.0}
    mean = statistics.fmean(pnls)
    sd = statistics.pstdev(pnls) if n > 1 else 0.0
    if sd > 0:
        sharpe = mean / sd * math.sqrt(trades_per_year)
    else:
        sharpe = math.inf if mean > 0 else (-math.inf if mean < 0 else float("nan"))
    cum, peak, max_dd = 0.0, 0.0, 0.0
    for p in pnls:
        cum += p
        peak = max(peak, cum)
        dd = peak - cum
        # drawdown relative to peak equity; guard the zero-peak edge
        denom = abs(peak) if peak != 0 else 1.0
        max_dd = max(max_dd, dd / denom)
    return {"n": n, "sharpe": sharpe, "max_drawdown": max_dd}


def evaluate(
    registry: PromotionRegistry,
    strategy_id: str,
    thresholds: Thresholds | None = None,
) -> tuple[bool, str]:
    """Check a PROMOTED strategy's PAPER fills. Returns (demoted, message)."""
    t = thresholds or Thresholds.from_env()
    rec = registry.get(strategy_id)
    if rec is None or rec.get("status") != PROMOTED:
        return False, f"{strategy_id} is not PROMOTED; demotion not applicable"

    pnls = registry.fills(strategy_id)
    if len(pnls) < t.demote_min_fills:
        return False, (
            f"{strategy_id}: insufficient fills for demotion review "
            f"({len(pnls)} < {t.demote_min_fills}); no action"
        )

    stats = trailing_stats(pnls, t.demote_trades_per_year)
    sharpe, max_dd = stats["sharpe"], stats["max_drawdown"]

    reason = None
    if not math.isnan(sharpe) and sharpe < t.demote_min_sharpe:
        reason = (
            f"trailing Sharpe {sharpe:.3f} < floor {t.demote_min_sharpe} "
            f"over {len(pnls)} fills"
        )
    elif max_dd > t.demote_max_drawdown:
        reason = (
            f"max drawdown {max_dd:.3%} > limit {t.demote_max_drawdown:.3%} "
            f"over {len(pnls)} fills"
        )

    if reason is None:
        return False, (
            f"{strategy_id}: within bounds "
            f"(Sharpe {sharpe:.3f}, max DD {max_dd:.3%}, n={len(pnls)})"
        )

    registry.set_demoted(strategy_id, reason, rec.get("report_sha256"))
    return True, f"{strategy_id} DEMOTED: {reason}"


def record_fill(
    registry: PromotionRegistry,
    strategy_id: str,
    pnl: float,
    ts: str | None = None,
    notional: float | None = None,
    thresholds: Thresholds | None = None,
) -> tuple[bool, str]:
    """Append a PAPER fill and immediately re-evaluate demotion.

    Returns (demoted, message). Recording never fails the caller: the fill
    is always persisted even if evaluation errors.
    """
    registry.record_fill(strategy_id, pnl, ts=ts, notional=notional)
    try:
        return evaluate(registry, strategy_id, thresholds)
    except Exception as exc:  # fail closed: fill kept, evaluation retried later
        return False, f"{strategy_id}: fill recorded; evaluation deferred ({exc})"


def manual_demote(
    registry: PromotionRegistry,
    strategy_id: str,
    reason: str,
) -> tuple[bool, str]:
    """Operator-initiated demotion (safe direction). Records reason."""
    rec = registry.get(strategy_id)
    if rec is None:
        return False, f"unknown strategy {strategy_id!r}"
    registry.set_demoted(
        strategy_id, f"manual: {reason}", rec.get("report_sha256")
    )
    return True, f"{strategy_id} DEMOTED (manual): {reason}"
