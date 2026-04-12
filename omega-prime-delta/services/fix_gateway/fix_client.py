"""FIX initiator used for outbound broker routing."""

from __future__ import annotations

import quickfix as fix
import quickfix44 as fix44


class PrimeBrokerFixClient:
    def __init__(self, fix_cfg_path: str) -> None:
        settings = fix.SessionSettings(fix_cfg_path)
        app = fix.Application()
        store_factory = fix.FileStoreFactory(settings)
        log_factory = fix.FileLogFactory(settings)
        self.initiator = fix.SocketInitiator(app, store_factory, settings, log_factory)

    def start(self) -> None:
        self.initiator.start()

    def stop(self) -> None:
        self.initiator.stop()

    def send_order(self, session_id, order: dict) -> None:
        message = fix44.NewOrderSingle()
        message.setField(fix.ClOrdID(order["clOrdID"]))
        message.setField(fix.Symbol(order["symbol"]))
        message.setField(fix.Side(fix.Side_BUY if order["side"] == "buy" else fix.Side_SELL))
        message.setField(fix.OrderQty(float(order["quantity"])))
        ord_type = fix.OrdType_LIMIT if order.get("price") else fix.OrdType_MARKET
        message.setField(fix.OrdType(ord_type))
        if order.get("price"):
            message.setField(fix.Price(float(order["price"])))
        fix.Session.sendToTarget(message, session_id)
