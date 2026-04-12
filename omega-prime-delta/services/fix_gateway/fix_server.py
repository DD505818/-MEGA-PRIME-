"""FIX 4.4 application wrapper.

If quickfix is unavailable, the module still imports and can be tested in dry-run mode.
"""

from __future__ import annotations

import uuid
from typing import Any, Dict

from .engine import FixOrderRouter

try:  # pragma: no cover - optional dependency
    import quickfix as fix
    import quickfix44 as fix44
except Exception:  # pragma: no cover
    fix = None
    fix44 = None


class OmegaPrimeFixApplication:
    def __init__(self, order_manager: Any):
        self.router = FixOrderRouter(order_manager)

    def onCreate(self, sessionID: Any) -> None:  # noqa: N802
        print(f"FIX session created: {sessionID}")

    def onLogon(self, sessionID: Any) -> None:  # noqa: N802
        print(f"Logon: {sessionID}")

    def process_new_order(self, order: Dict[str, Any]) -> Dict[str, Any]:
        result = self.router.route(order)
        return {
            "order_id": result.get("order_id", str(uuid.uuid4())),
            "symbol": result.get("symbol", order.get("symbol")),
            "filled_qty": result.get("filled_qty", order.get("orderQty", 0.0)),
            "avg_price": result.get("avg_price", order.get("price", 0.0) or 0.0),
            "status": result.get("status", "FILLED"),
        }

    def send_execution_report(self, sessionID: Any, order_result: Dict[str, Any]) -> None:
        if not (fix and fix44):
            print(f"dry-run execution report: {order_result}")
            return

        report = fix44.ExecutionReport()
        report.setField(fix.OrderID(order_result["order_id"]))
        report.setField(fix.ExecID(str(uuid.uuid4())))
        report.setField(fix.ExecType(fix.ExecType_FILL))
        report.setField(fix.OrdStatus(fix.OrdStatus_FILLED))
        report.setField(fix.Symbol(order_result["symbol"]))
        report.setField(fix.LastQty(float(order_result["filled_qty"])))
        report.setField(fix.LastPx(float(order_result["avg_price"])))
        fix.Session.sendToTarget(report, sessionID)
