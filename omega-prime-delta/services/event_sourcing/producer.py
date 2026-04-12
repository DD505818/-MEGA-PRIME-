"""Event producer with idempotent publishing semantics.

Uses confluent-kafka when available and falls back to in-memory sink for local tests.
"""

from __future__ import annotations

import json
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Dict, Optional, Set

from .topics import RISK_TOPIC, TRADES_TOPIC, SUPPORTED_EVENT_TYPES


try:  # pragma: no cover - optional dependency
    from confluent_kafka import Producer as KafkaProducer
except Exception:  # pragma: no cover
    KafkaProducer = None


@dataclass
class EventProducer:
    bootstrap_servers: str = "kafka:9092"

    def __post_init__(self) -> None:
        self.idempotency_store: Set[str] = set()
        self.local_messages: list[tuple[str, str, str]] = []
        self.producer = (
            KafkaProducer({"bootstrap.servers": self.bootstrap_servers})
            if KafkaProducer
            else None
        )

    def _publish(self, topic: str, key: str, event: Dict[str, Any]) -> None:
        payload = json.dumps(event)
        if self.producer is not None:
            self.producer.produce(topic, key=key, value=payload)
            self.producer.flush()
        else:
            self.local_messages.append((topic, key, payload))

    def publish_order(
        self,
        order: Dict[str, Any],
        event_type: str = "ORDER_PLACED",
        event_id: Optional[str] = None,
    ) -> Optional[str]:
        if event_type not in SUPPORTED_EVENT_TYPES:
            raise ValueError(f"unsupported event_type={event_type}")

        event_id = event_id or str(uuid.uuid4())
        if event_id in self.idempotency_store:
            return None

        event = {
            "event_id": event_id,
            "event_type": event_type,
            "aggregate_id": order.get("id", "unknown"),
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "data": order,
            "version": 1,
        }
        self._publish(TRADES_TOPIC, str(order.get("id", event_id)), event)
        self.idempotency_store.add(event_id)
        return event_id

    def publish_fill(self, fill: Dict[str, Any]) -> Optional[str]:
        return self.publish_order(fill, event_type="ORDER_FILLED")

    def publish_risk_event(self, risk_check: Dict[str, Any]) -> str:
        event_id = str(uuid.uuid4())
        event = {
            "event_id": event_id,
            "event_type": "RISK_CHECK",
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "data": risk_check,
            "version": 1,
        }
        self._publish(RISK_TOPIC, event_id, event)
        self.idempotency_store.add(event_id)
        return event_id
