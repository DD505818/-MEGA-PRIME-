"""Signal normalizer — converts raw agent output to canonical signal contract."""
from __future__ import annotations
import os
import time
import uuid
from typing import Optional


PAPER_MODE = os.getenv("PAPER_MODE", "true").lower() == "true"


def normalize_signal(
    strategy_output: Optional[dict],
    strategy_name: str,
    symbol: str = "BTCUSDT",
    base_qty: float = 0.01,
    equity: float = 100_000.0,
) -> Optional[dict]:
    if not strategy_output:
        return None

    side = strategy_output.get("side", "BUY")
    if side not in ("BUY", "SELL", "MAKER"):
        return None

    price = float(strategy_output.get("entry", 0))
    if price <= 0:
        return None

    confidence = float(strategy_output.get("confidence", 0.7))

    # Kelly-fractional size: risk_per_trade * equity / |entry - stop|
    risk_per_trade = 0.005
    stop = strategy_output.get("stop")
    if stop is not None:
        try:
            risk_distance = abs(price - float(stop))
            if risk_distance > 0:
                qty = (risk_per_trade * equity) / risk_distance
                qty = round(max(base_qty, min(qty, equity * 0.02 / price)), 6)
            else:
                qty = base_qty
        except (TypeError, ValueError):
            qty = base_qty
    else:
        qty = base_qty

    return {
        "signal_id": str(uuid.uuid4()),
        "strategy_id": strategy_name,
        "symbol": strategy_output.get("symbol", symbol),
        "side": side,
        "quantity": float(qty),
        "limit_price": float(price),
        "stop": float(strategy_output.get("stop", price * 0.99)),
        "target": float(strategy_output.get("target", price * 1.01)),
        "confidence": float(confidence),
        "timestamp": int(time.time() * 1000),
        "mode": "paper" if PAPER_MODE else "live",
        "reason": str(strategy_output.get("reason", f"{strategy_name}:{side}")),
        "meta": {
            k: v for k, v in strategy_output.items()
            if k not in ("side", "entry", "stop", "target", "confidence", "reason")
        },
    }
