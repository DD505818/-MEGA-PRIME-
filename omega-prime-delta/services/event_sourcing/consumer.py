"""Replay consumer used to reconstruct portfolio state from Kafka."""

from __future__ import annotations

import json
from collections.abc import Callable

from confluent_kafka import Consumer, KafkaError

from .topics import TRADE_EVENTS_TOPIC


class PortfolioReconstructor:
    def __init__(
        self,
        update_from_fill: Callable[[dict], None],
        clear_positions: Callable[[], None],
        bootstrap_servers: str = "kafka:9092",
    ) -> None:
        self._update_from_fill = update_from_fill
        self._clear_positions = clear_positions
        self.consumer = Consumer(
            {
                "bootstrap.servers": bootstrap_servers,
                "group.id": "portfolio-rebuilder",
                "auto.offset.reset": "earliest",
                "enable.auto.commit": False,
            }
        )
        self.consumer.subscribe([TRADE_EVENTS_TOPIC])

    def rebuild_from_scratch(self) -> None:
        self._clear_positions()

        while True:
            message = self.consumer.poll(1.0)
            if message is None:
                break
            if message.error():
                if message.error().code() == KafkaError._PARTITION_EOF:
                    continue
                raise RuntimeError(f"kafka error: {message.error()}")

            event = json.loads(message.value())
            self._apply_event(event)

        self.consumer.commit()

    def _apply_event(self, event: dict) -> None:
        event_type = event.get("event_type")
        if event_type == "ORDER_FILLED":
            self._update_from_fill(event["data"])
