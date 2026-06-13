"""Shared utilities for strategy agents.

Agents emit trade candidates only; AEGIS remains the sole order approval layer.
"""
from __future__ import annotations

from typing import Any, Mapping, Optional

import numpy as np

STRATEGY_VERSION = "14.2.1"


def safe_float(value: Any, default: Optional[float] = None) -> Optional[float]:
    try:
        value = float(value)
    except (TypeError, ValueError):
        return default
    if not np.isfinite(value):
        return default
    return value


def finite_positive(value: Any) -> bool:
    parsed = safe_float(value)
    return parsed is not None and parsed > 0


def clamp(value: float, lo: float, hi: float) -> float:
    return float(max(lo, min(hi, value)))


def calc_atr(df, period: int = 14) -> float:
    if df is None or len(df) < period + 1:
        return float("nan")
    high = df["high"].astype(float)
    low = df["low"].astype(float)
    close = df["close"].astype(float)
    tr1 = high - low
    tr2 = np.abs(high - close.shift())
    tr3 = np.abs(low - close.shift())
    tr = np.maximum(np.maximum(tr1, tr2), tr3)
    return float(tr.rolling(period).mean().iloc[-1])


def infer_symbol(*frames) -> str:
    for frame in frames:
        if frame is None:
            continue
        attrs = getattr(frame, "attrs", {}) or {}
        symbol = attrs.get("symbol") or attrs.get("asset")
        if symbol:
            return str(symbol)
    return "UNKNOWN"


def build_signal(
    *,
    agent: str,
    side: str,
    entry: float,
    stop: float,
    target: float,
    risk: float,
    confidence: float,
    reason: str,
    symbol: str = "UNKNOWN",
    timeframe: str = "UNKNOWN",
    extra: Optional[Mapping[str, Any]] = None,
) -> Optional[dict]:
    if side not in {"BUY", "SELL", "MAKER", "EXIT"}:
        return None
    if not finite_positive(entry):
        return None
    if risk < 0 or not np.isfinite(risk):
        return None
    payload = {
        "agent": agent,
        "symbol": symbol,
        "timeframe": timeframe,
        "side": side,
        "direction": side,
        "entry": float(entry),
        "stop": float(stop),
        "target": float(target),
        "risk": float(risk),
        "confidence": clamp(float(confidence), 0.0, 1.0),
        "reason": reason,
        "strategy_version": STRATEGY_VERSION,
    }
    if extra:
        payload.update(dict(extra))
    return payload
