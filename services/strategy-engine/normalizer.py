from __future__ import annotations

from typing import Any, Dict


REQUIRED_FIELDS = {
    "order_id",
    "symbol",
    "side",
    "quantity",
    "limit_price",
    "timestamp",
    "strategy",
}


def normalize_signal(signal: Dict[str, Any]) -> Dict[str, Any]:
    normalized = {
        "order_id": str(signal.get("order_id", "")),
        "symbol": str(signal.get("symbol", "")).upper(),
        "side": str(signal.get("side", "")).upper(),
        "quantity": float(signal.get("quantity", 0.0)),
        "limit_price": float(signal.get("limit_price", 0.0)),
        "timestamp": signal.get("timestamp"),
        "strategy": str(signal.get("strategy", "unknown")),
    }

    missing = [field for field in REQUIRED_FIELDS if normalized.get(field) in (None, "")]
    if missing:
        raise ValueError(f"signal missing required fields: {missing}")

    if normalized["side"] not in {"BUY", "SELL"}:
        raise ValueError("side must be BUY or SELL")
    if normalized["quantity"] <= 0:
        raise ValueError("quantity must be positive")
    if normalized["limit_price"] <= 0:
        raise ValueError("limit_price must be positive")

    return normalized
