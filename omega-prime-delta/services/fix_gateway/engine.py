"""FIX translation and routing helpers."""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Dict


@dataclass
class FixOrderRouter:
    order_manager: Any

    def fix_to_internal(self, order: Dict[str, Any]) -> Dict[str, Any]:
        return {
            "client_order_id": order.get("clOrdID"),
            "symbol": order.get("symbol"),
            "side": "buy" if str(order.get("side")) in {"1", "buy", "BUY"} else "sell",
            "quantity": float(order.get("orderQty", 0.0)),
            "price": order.get("price"),
            "order_type": "limit" if order.get("price") is not None else "market",
        }

    def route(self, order: Dict[str, Any]) -> Dict[str, Any]:
        internal = self.fix_to_internal(order)
        return self.order_manager.place_order(internal)
