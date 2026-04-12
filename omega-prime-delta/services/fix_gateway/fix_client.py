"""Outbound FIX client adapter."""

from __future__ import annotations

from typing import Any, Dict

try:  # pragma: no cover - optional dependency
    import quickfix as fix
    import quickfix44 as fix44
except Exception:  # pragma: no cover
    fix = None
    fix44 = None


class PrimeBrokerFixClient:
    def __init__(self, broker_config: Dict[str, Any]):
        self.broker_config = broker_config
        self.session = self.create_session(broker_config)

    def create_session(self, config: Dict[str, Any]) -> Any:
        if not (fix and fix44):
            return None
        settings = fix.SessionSettings(config["fix_cfg_path"])
        app = fix.Application()
        store_factory = fix.FileStoreFactory(settings)
        log_factory = fix.FileLogFactory(settings)
        return fix.SocketInitiator(app, store_factory, settings, log_factory)

    def send_order(self, order: Dict[str, Any]) -> Dict[str, Any]:
        if not (fix and fix44 and self.session):
            return {"status": "queued", "order": order}

        message = fix44.NewOrderSingle()
        message.setField(fix.ClOrdID(order["clOrdID"]))
        message.setField(fix.Symbol(order["symbol"]))
        message.setField(
            fix.Side(fix.Side_BUY if order["side"] == "buy" else fix.Side_SELL)
        )
        message.setField(fix.OrderQty(float(order["quantity"])))
        message.setField(
            fix.OrdType(fix.OrdType_LIMIT if order.get("price") else fix.OrdType_MARKET)
        )
        if order.get("price") is not None:
            message.setField(fix.Price(float(order["price"])))
        fix.Session.sendToTarget(message, self.session.getSessionID())
        return {"status": "sent", "order_id": order["clOrdID"]}
