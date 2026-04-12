"""Replay consumer for portfolio reconstruction."""

from __future__ import annotations

import json
from typing import Any, Iterable, Protocol

try:  # pragma: no cover - optional dependency
    from confluent_kafka import Consumer as KafkaConsumer, KafkaError
except Exception:  # pragma: no cover
    KafkaConsumer = None
    KafkaError = None

from .topics import TRADES_TOPIC


class PositionTracker(Protocol):
    positions: dict[str, Any]

    def update_from_fill(self, fill: dict[str, Any]) -> None: ...


class PortfolioReconstructor:
    def __init__(self, position_tracker: PositionTracker, bootstrap_servers: str = "kafka:9092"):
        self.tracker = position_tracker
        self.bootstrap_servers = bootstrap_servers
        self.local_events: list[dict[str, Any]] = []
        if KafkaConsumer:
            self.consumer = KafkaConsumer(
                {
                    "bootstrap.servers": bootstrap_servers,
                    "group.id": "portfolio-rebuilder",
                    "auto.offset.reset": "earliest",
                    "enable.auto.commit": False,
                }
            )
            self.consumer.subscribe([TRADES_TOPIC])
        else:
            self.consumer = None

    def set_local_events(self, events: Iterable[dict[str, Any]]) -> None:
        self.local_events = list(events)

    def rebuild_from_scratch(self) -> int:
        self.tracker.positions.clear()
        applied = 0
        if self.consumer is None:
            for event in self.local_events:
                self._apply_event(event)
                applied += 1
            return applied

        while True:  # pragma: no cover - integration path
            msg = self.consumer.poll(1.0)
            if msg is None:
                break
            if msg.error():
                if KafkaError and msg.error().code() == KafkaError._PARTITION_EOF:
                    continue
                raise RuntimeError(msg.error())
            event = json.loads(msg.value())
            self._apply_event(event)
            applied += 1

        self.consumer.commit()
        return applied

    def _apply_event(self, event: dict[str, Any]) -> None:
        event_type = event.get("event_type")
        if event_type == "ORDER_FILLED":
            self.tracker.update_from_fill(event.get("data", {}))
        elif event_type == "ORDER_PLACED":
            pass
