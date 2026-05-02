"""Signal validator — 14-gate pre-emit validation.

Every signal from every agent passes through validate() before reaching
the AEGIS risk engine. Gates are ordered from cheapest to most expensive.
"""
from __future__ import annotations
import os
import time
import uuid
from typing import Optional, Tuple

REQUIRED_KEYS = {
    "signal_id", "strategy_id", "symbol", "side",
    "quantity", "limit_price", "confidence", "timestamp", "mode",
}
VALID_SIDES = {"BUY", "SELL", "MAKER"}
VALID_MODES = {"paper", "live"}
LIVE_MODE = os.getenv("PAPER_MODE", "true").lower() != "true"


def validate(signal: dict) -> Tuple[bool, str]:
    """Run all 14 validation gates. Returns (ok, reject_reason)."""

    # Gate 1 — schema completeness
    missing = REQUIRED_KEYS - set(signal.keys())
    if missing:
        return False, f"SCHEMA_MISSING_KEYS:{missing}"

    # Gate 2 — side value
    if signal["side"] not in VALID_SIDES:
        return False, f"INVALID_SIDE:{signal['side']}"

    # Gate 3 — mode consistency with env
    signal_mode = signal.get("mode", "paper")
    if signal_mode not in VALID_MODES:
        return False, f"INVALID_MODE:{signal_mode}"
    if signal_mode == "live" and not LIVE_MODE:
        return False, "LIVE_SIGNAL_IN_PAPER_ENV"
    if signal_mode == "paper" and LIVE_MODE:
        signal["mode"] = "live"  # promote to live in live env

    # Gate 4 — UUID format
    try:
        uuid.UUID(str(signal["signal_id"]))
    except (ValueError, AttributeError):
        return False, "INVALID_UUID"

    # Gate 5 — price sanity
    price = signal.get("limit_price", 0)
    try:
        price = float(price)
    except (TypeError, ValueError):
        return False, "INVALID_PRICE_TYPE"
    if price <= 0:
        return False, "PRICE_NON_POSITIVE"

    # Gate 6 — quantity sanity
    qty = signal.get("quantity", 0)
    try:
        qty = float(qty)
    except (TypeError, ValueError):
        return False, "INVALID_QTY_TYPE"
    if qty <= 0:
        return False, "QTY_NON_POSITIVE"

    # Gate 7 — confidence range
    conf = signal.get("confidence", 0)
    try:
        conf = float(conf)
    except (TypeError, ValueError):
        return False, "INVALID_CONFIDENCE_TYPE"
    if not (0.0 < conf <= 1.0):
        return False, f"CONFIDENCE_OUT_OF_RANGE:{conf}"

    # Gate 8 — minimum confidence threshold (strategy-level)
    if conf < 0.60:
        return False, f"CONFIDENCE_BELOW_MIN:{conf:.2f}"

    # Gate 9 — timestamp recency (reject stale signals older than 30s)
    ts = signal.get("timestamp", 0)
    now_ms = int(time.time() * 1000)
    try:
        ts = int(ts)
    except (TypeError, ValueError):
        return False, "INVALID_TIMESTAMP"
    if abs(now_ms - ts) > 30_000:
        return False, f"STALE_SIGNAL_AGE:{(now_ms - ts) // 1000}s"

    # Gate 10 — symbol non-empty
    sym = signal.get("symbol", "")
    if not sym or not isinstance(sym, str):
        return False, "INVALID_SYMBOL"

    # Gate 11 — strategy_id non-empty
    strat = signal.get("strategy_id", "")
    if not strat or not isinstance(strat, str):
        return False, "INVALID_STRATEGY_ID"

    # Gate 12 — notional cap pre-check (hard limit $500K)
    notional = price * qty
    if notional > 500_000:
        return False, f"NOTIONAL_EXCEEDS_HARD_CAP:{notional:.0f}"

    # Gate 13 — stop/target direction consistency (if provided)
    stop = signal.get("stop")
    target = signal.get("target")
    if stop is not None and target is not None:
        try:
            stop = float(stop)
            target = float(target)
            if signal["side"] == "BUY":
                if stop >= price:
                    return False, "BUY_STOP_ABOVE_ENTRY"
                if target <= price:
                    return False, "BUY_TARGET_BELOW_ENTRY"
            elif signal["side"] == "SELL":
                if stop <= price:
                    return False, "SELL_STOP_BELOW_ENTRY"
                if target >= price:
                    return False, "SELL_TARGET_ABOVE_ENTRY"
        except (TypeError, ValueError):
            return False, "INVALID_STOP_TARGET_TYPE"

    # Gate 14 — reason string present
    reason = signal.get("reason", "")
    if not reason or not isinstance(reason, str):
        signal["reason"] = f"auto:{signal['strategy_id']}:{signal['side']}"

    return True, "APPROVED"
