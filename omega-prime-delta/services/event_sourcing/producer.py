"""Kafka event producer for order, fill, and risk lifecycle events."""

from __future__ import annotations

import json
import uuid
from datetime import datetime, timezone

from confluent_kafka import Producer

from .topics import RISK_EVENTS_TOPIC, TRADE_EVENTS_TOPIC


class EventProducer:
    def __init__(self, bootstrap_servers: str = "kafka:9092") -> None:
        self.producer = Producer({"bootstrap.servers": bootstrap_servers})
        self.idempotency_store: set[str] = set()

    def _build_event(self, aggregate_id: str, payload: dict, event_type: str) -> dict:
        return {
            "event_id": str(uuid.uuid4()),
            "event_type": event_type,
            "aggregate_id": aggregate_id,
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "data": payload,
            "version": 1,
        }

    def publish_order(self, order: dict, event_type: str = "ORDER_PLACED") -> str | None:
        event = self._build_event(order["id"], order, event_type)
        event_id = event["event_id"]
        if event_id in self.idempotency_store:
            return None

        self.producer.produce(
            TRADE_EVENTS_TOPIC,
            key=str(order["id"]),
            value=json.dumps(event).encode("utf-8"),
        )
        self.producer.flush()
        self.idempotency_store.add(event_id)
        return event_id

    def publish_fill(self, fill: dict) -> str | None:
        return self.publish_order(fill, event_type="ORDER_FILLED")

    def publish_risk_event(self, risk_check: dict) -> None:
        event = self._build_event(risk_check.get("id", "risk-check"), risk_check, "RISK_CHECK")
        self.producer.produce(RISK_EVENTS_TOPIC, value=json.dumps(event).encode("utf-8"))
        self.producer.flush()
