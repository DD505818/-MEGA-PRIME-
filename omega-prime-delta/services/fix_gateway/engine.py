"""FIX 4.4 translation helpers."""

from __future__ import annotations


def fix_side_to_internal(side: str) -> str:
    return "buy" if side == "1" else "sell"


def normalize_fix_order(order: dict) -> dict:
    return {
        "client_order_id": order["cl_ord_id"],
        "symbol": order["symbol"],
        "side": fix_side_to_internal(order["side"]),
        "quantity": float(order["order_qty"]),
        "price": float(order["price"]) if order.get("price") else None,
        "order_type": order.get("ord_type", "1"),
    }
