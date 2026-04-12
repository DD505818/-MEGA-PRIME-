"""FIX acceptor application for inbound prime-broker connectivity."""

from __future__ import annotations

import uuid

import quickfix as fix
import quickfix44 as fix44

from .engine import normalize_fix_order


class OmegaPrimeFixApplication(fix.Application):
    def __init__(self, order_manager) -> None:
        super().__init__()
        self.order_manager = order_manager

    def onCreate(self, session_id):
        print(f"FIX session created: {session_id}")

    def onLogon(self, session_id):
        print(f"FIX logon: {session_id}")

    def fromApp(self, message, session_id):
        msg_type = fix.MsgType()
        message.getHeader().getField(msg_type)
        if msg_type.getValue() == fix.MsgType_NewOrderSingle:
            self._process_new_order(message, session_id)

    def _process_new_order(self, message, session_id):
        order = {
            "cl_ord_id": self._field(message, fix.ClOrdID()),
            "symbol": self._field(message, fix.Symbol()),
            "side": self._field(message, fix.Side()),
            "order_qty": self._field(message, fix.OrderQty()),
            "ord_type": self._field(message, fix.OrdType()),
            "price": self._field(message, fix.Price()) if message.isSetField(fix.Price().getField()) else None,
        }

        internal_order = normalize_fix_order(order)
        result = self.order_manager.place_order(internal_order)
        self._send_execution_report(session_id, result)

    def _send_execution_report(self, session_id, order_result):
        report = fix44.ExecutionReport()
        report.setField(fix.OrderID(str(order_result["order_id"])))
        report.setField(fix.ExecID(str(uuid.uuid4())))
        report.setField(fix.ExecType(fix.ExecType_FILL))
        report.setField(fix.OrdStatus(fix.OrdStatus_FILLED))
        report.setField(fix.Symbol(order_result["symbol"]))
        report.setField(fix.LastQty(float(order_result["filled_qty"])))
        report.setField(fix.LastPx(float(order_result["avg_price"])))
        fix.Session.sendToTarget(report, session_id)

    @staticmethod
    def _field(message, field):
        message.getField(field)
        return field.getValue()
