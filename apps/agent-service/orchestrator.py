"""Orchestrator — drives all 19 agents on incoming feature events."""
from __future__ import annotations
import asyncio
import json
import logging
import os
import time

import pandas as pd
from aiokafka import AIOKafkaConsumer, AIOKafkaProducer

from normalizer import normalize_signal
from signal_validator import validate

log = logging.getLogger("orchestrator")
KAFKA = os.getenv("KAFKA_BROKERS", "kafka:9092")
EQUITY = float(os.getenv("PORTFOLIO_EQUITY", "100000"))


def _build_df(records: list[dict]) -> pd.DataFrame:
    if not records:
        return pd.DataFrame(columns=["open", "high", "low", "close", "volume"])
    df = pd.DataFrame(records)
    for col in ("open", "high", "low", "close"):
        if col in df.columns:
            df[col] = df[col].astype(float)
    return df


class Orchestrator:
    def __init__(self, strategies: list):
        self.strategies = {s.name: s for s in strategies}
        self._daily_df: pd.DataFrame = pd.DataFrame()
        self._intraday_df: pd.DataFrame = pd.DataFrame()

    async def run(self) -> None:
        producer = AIOKafkaProducer(bootstrap_servers=KAFKA)
        consumer = AIOKafkaConsumer(
            "features.norm",
            bootstrap_servers=KAFKA,
            group_id="agent-service",
            auto_offset_reset="latest",
        )
        await producer.start()
        await consumer.start()

        log.info("Orchestrator started with %d strategies", len(self.strategies))
        try:
            async for msg in consumer:
                await self._handle(msg, producer)
        finally:
            await consumer.stop()
            await producer.stop()

    async def _handle(self, msg, producer: AIOKafkaProducer) -> None:
        try:
            payload = json.loads(msg.value)
        except Exception:
            return

        # Update market DataFrames from feature engine payload
        if "daily_bars" in payload:
            self._daily_df = _build_df(payload["daily_bars"])
        if "intraday_bars" in payload:
            self._intraday_df = _build_df(payload["intraday_bars"])

        if self._daily_df.empty:
            return

        # Collect all agent signals (needed by NEXUS meta-fusion)
        raw_signals: dict[str, dict | None] = {}
        for name, strat in self.strategies.items():
            if name == "NEXUS":
                continue
            try:
                kwargs = {}
                # Inject optional data from payload
                if hasattr(strat, "generate_signal"):
                    import inspect
                    sig = inspect.signature(strat.generate_signal)
                    params = list(sig.parameters.keys())
                    if "order_book" in params or "venue_books" in params:
                        kwargs["venue_books"] = payload.get("venue_books")
                        kwargs["order_book"] = payload.get("order_book")
                    if "sentiment_data" in params:
                        kwargs["sentiment_data"] = payload.get("sentiment_data")
                    if "macro_data" in params:
                        kwargs["macro_data"] = payload.get("macro_data")
                    if "options_data" in params:
                        kwargs["options_data"] = payload.get("options_data")
                    if "yield_data" in params:
                        kwargs["yield_data"] = payload.get("yield_data")
                    if "pair_df" in params:
                        pair_records = payload.get("pair_bars", [])
                        kwargs["pair_df"] = _build_df(pair_records) if pair_records else None

                raw = strat.generate_signal(self._daily_df, self._intraday_df, **kwargs)
                raw_signals[name] = raw
            except Exception as exc:
                log.warning("Agent %s raised: %s", name, exc)
                raw_signals[name] = None

        # NEXUS receives peer signals as agent_signals
        if "NEXUS" in self.strategies:
            try:
                nexus = self.strategies["NEXUS"]
                raw_signals["NEXUS"] = nexus.generate_signal(
                    self._daily_df, self._intraday_df,
                    agent_signals=raw_signals,
                )
            except Exception as exc:
                log.warning("NEXUS raised: %s", exc)
                raw_signals["NEXUS"] = None

        # Normalize, validate, and publish
        emitted = 0
        for name, raw in raw_signals.items():
            signal = normalize_signal(raw, name, equity=EQUITY)
            if signal is None:
                continue
            ok, reason = validate(signal)
            if not ok:
                log.debug("Signal from %s rejected: %s", name, reason)
                continue
            try:
                await producer.send(
                    "signals.raw",
                    json.dumps(signal).encode(),
                )
                emitted += 1
            except Exception as exc:
                log.error("Failed to publish signal from %s: %s", name, exc)

        if emitted:
            log.info("Emitted %d signals from %d agents", emitted, len(self.strategies))
