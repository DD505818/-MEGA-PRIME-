"""LLM Service — Groq-powered strategy explanation and signal narration.

Provides natural-language explanations of signals and strategy decisions
via Groq's API (default model: llama-3.3-70b-versatile for <1s latency).
"""
import json
import os
import asyncio
import logging
from typing import Optional

import redis.asyncio as aioredis
from aiokafka import AIOKafkaConsumer, AIOKafkaProducer
import httpx

log = logging.getLogger("llm-service")
logging.basicConfig(level=logging.INFO)

GROQ_API_KEY = os.getenv("GROQ_API_KEY", "")
GROQ_URL = "https://api.groq.com/openai/v1/chat/completions"
GROQ_MODEL = os.getenv("GROQ_MODEL", "llama-3.3-70b-versatile")
KAFKA = os.getenv("KAFKA_BROKERS", "kafka:9092")
REDIS_URL = os.getenv("REDIS_URL", "redis://redis:6379")

SYSTEM_PROMPT = """You are ΩMEGA PRIME Δ, an elite algorithmic trading AI.
Explain trading signals in 1-2 sentences: what the signal is, why it triggered,
and the key risk. Be precise and professional. Never invent data."""


async def explain_signal(signal: dict, client: httpx.AsyncClient) -> Optional[str]:
    if not GROQ_API_KEY:
        return f"Signal {signal.get('strategy_id')} {signal.get('side')} {signal.get('symbol')}"
    try:
        resp = await client.post(
            GROQ_URL,
            headers={"Authorization": f"Bearer {GROQ_API_KEY}"},
            json={
                "model": GROQ_MODEL,
                "messages": [
                    {"role": "system", "content": SYSTEM_PROMPT},
                    {"role": "user", "content": f"Explain this signal: {json.dumps(signal, indent=2)}"},
                ],
                "max_tokens": 120,
                "temperature": 0.3,
            },
            timeout=5.0,
        )
        data = resp.json()
        return data["choices"][0]["message"]["content"].strip()
    except Exception as exc:
        log.warning("LLM explain error: %s", exc)
        return None


async def main():
    rdb = aioredis.from_url(REDIS_URL)
    producer = AIOKafkaProducer(bootstrap_servers=KAFKA)
    consumer = AIOKafkaConsumer(
        "signals.approved",
        bootstrap_servers=KAFKA,
        group_id="llm-service",
        auto_offset_reset="latest",
    )
    await producer.start()
    await consumer.start()

    async with httpx.AsyncClient() as client:
        async for msg in consumer:
            try:
                signal = json.loads(msg.value)
                explanation = await explain_signal(signal, client)
                if explanation:
                    signal["llm_explanation"] = explanation
                    key = f"signal:explain:{signal.get('signal_id', 'unknown')}"
                    await rdb.setex(key, 3600, explanation)
                    await producer.send(
                        "signals.explained",
                        json.dumps(signal).encode(),
                    )
            except Exception as exc:
                log.error("LLM service error: %s", exc)

    await consumer.stop()
    await producer.stop()


if __name__ == "__main__":
    asyncio.run(main())
