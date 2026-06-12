"""LLM Service — Groq signal narration + Kimi K2 AI Strategist chat.

Two responsibilities:
  1. Kafka consumer: explain approved signals via Groq (background task).
  2. HTTP server:    streaming Kimi K2 chat for the AI Strategist UI.
"""
from __future__ import annotations

import asyncio
import json
import logging
import os
import time
from typing import Optional

import redis.asyncio as aioredis
import uvicorn
from fastapi import FastAPI, Header, HTTPException, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import StreamingResponse
from pydantic import BaseModel, Field
import httpx

from kimi_service import stream_chat, check_kill_switch, check_spend_cap

log = logging.getLogger("llm-service")
logging.basicConfig(level=logging.INFO)

# ── Groq config (legacy signal-narration path) ────────────────────────────────
GROQ_API_KEY = os.getenv("GROQ_API_KEY", "")
GROQ_URL = "https://api.groq.com/openai/v1/chat/completions"
GROQ_MODEL = os.getenv("GROQ_MODEL", "llama-3.3-70b-versatile")
KAFKA = os.getenv("KAFKA_BROKERS", "kafka:9092")
REDIS_URL = os.getenv("REDIS_URL", "redis://redis:6379")

GROQ_SYSTEM_PROMPT = """You are ΩMEGA PRIME Δ, an elite algorithmic trading AI.
Explain trading signals in 1-2 sentences: what the signal is, why it triggered,
and the key risk. Be precise and professional. Never invent data."""

# ── FastAPI app ───────────────────────────────────────────────────────────────
app = FastAPI(title="omega-prime-llm", version="3.0.0")
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)

_rdb: aioredis.Redis | None = None


async def get_redis() -> aioredis.Redis:
    global _rdb
    if _rdb is None:
        _rdb = aioredis.from_url(REDIS_URL)
    return _rdb


# ── Request / Response schemas ────────────────────────────────────────────────
class ChatMessage(BaseModel):
    role: str = Field(..., pattern="^(user|assistant)$")
    content: str = Field(..., min_length=1, max_length=8000)


class ChatRequest(BaseModel):
    messages: list[ChatMessage] = Field(..., min_length=1, max_length=20)
    user_id: str = Field(default="operator", max_length=64)
    max_tokens: int = Field(default=1024, ge=64, le=4096)


# ── HTTP endpoints ────────────────────────────────────────────────────────────
@app.get("/health")
async def health():
    rdb = await get_redis()
    kill = await check_kill_switch(rdb)
    _, spent = await check_spend_cap(rdb, "operator")
    return {
        "service": "omega-prime-llm",
        "version": "3.0.0",
        "kill_switch": kill,
        "daily_spend_usd": round(spent, 4),
    }


@app.post("/api/v1/llm/chat")
async def chat_stream(req: ChatRequest, x_user_id: str = Header(default="operator")):
    """
    Stream a Kimi K2 response as Server-Sent Events.

    The caller must handle `text/event-stream`:
      data: <text chunk>\\n\\n
      data: [DONE]\\n\\n
      data: [ERROR] <message>\\n\\n
    """
    rdb = await get_redis()

    async def _generate():
        try:
            messages = [m.model_dump() for m in req.messages]
            async for chunk in stream_chat(
                rdb,
                user_id=x_user_id,
                messages=messages,
                max_tokens=req.max_tokens,
            ):
                yield f"data: {json.dumps({'text': chunk})}\n\n"
            yield "data: [DONE]\n\n"
        except PermissionError as exc:
            yield f"data: [ERROR] {exc}\n\n"
        except Exception as exc:
            # Never include raw exception details that might carry key material
            log.error("Chat error for user %s: %s", x_user_id, type(exc).__name__)
            yield "data: [ERROR] LLM service error — see server logs\n\n"

    return StreamingResponse(_generate(), media_type="text/event-stream")


@app.get("/api/v1/llm/status")
async def llm_status(x_user_id: str = Header(default="operator")):
    """Return kill-switch state and per-user daily spend."""
    rdb = await get_redis()
    kill = await check_kill_switch(rdb)
    within_cap, spent = await check_spend_cap(rdb, x_user_id)
    daily_cap = float(os.getenv("LLM_DAILY_SPEND_CAP_USD", "5.0"))
    return {
        "kill_switch": kill,
        "daily_spend_usd": round(spent, 4),
        "daily_cap_usd": daily_cap,
        "within_cap": within_cap,
    }


@app.post("/api/v1/llm/explain-trade")
async def explain_trade(
    request: Request,
    x_user_id: str = Header(default="operator"),
):
    """
    Deep-link endpoint: accepts a trade object and returns a streaming
    natural-language explanation.  Called from the dashboard trade table.
    """
    body = await request.json()
    trade_json = json.dumps(body, indent=2)
    req = ChatRequest(
        messages=[
            ChatMessage(
                role="user",
                content=(
                    f"Please explain this trade in 2-3 sentences — what the strategy "
                    f"detected, why risk management approved it, and what to watch:\n\n"
                    f"{trade_json}"
                ),
            )
        ],
        user_id=x_user_id,
        max_tokens=512,
    )
    return await chat_stream(req, x_user_id=x_user_id)


# ── Groq Kafka consumer (background task) ────────────────────────────────────
async def _explain_signal(signal: dict, client: httpx.AsyncClient) -> Optional[str]:
    if not GROQ_API_KEY:
        return f"Signal {signal.get('strategy_id')} {signal.get('side')} {signal.get('symbol')}"
    try:
        resp = await client.post(
            GROQ_URL,
            headers={"Authorization": f"Bearer {GROQ_API_KEY}"},
            json={
                "model": GROQ_MODEL,
                "messages": [
                    {"role": "system", "content": GROQ_SYSTEM_PROMPT},
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
        log.warning("Groq explain error: %s", type(exc).__name__)
        return None


async def _kafka_consumer_task():
    """Run the Groq-powered signal narration consumer in background."""
    try:
        from aiokafka import AIOKafkaConsumer, AIOKafkaProducer
    except ImportError:
        log.warning("aiokafka not available — Kafka consumer disabled")
        return

    rdb = await get_redis()
    producer = AIOKafkaProducer(bootstrap_servers=KAFKA)
    consumer = AIOKafkaConsumer(
        "signals.approved",
        bootstrap_servers=KAFKA,
        group_id="llm-service",
        auto_offset_reset="latest",
    )
    try:
        await producer.start()
        await consumer.start()
    except Exception as exc:
        log.warning("Kafka unavailable — consumer not started: %s", type(exc).__name__)
        return

    async with httpx.AsyncClient() as client:
        async for msg in consumer:
            try:
                signal = json.loads(msg.value)
                explanation = await _explain_signal(signal, client)
                if explanation:
                    signal["llm_explanation"] = explanation
                    key = f"signal:explain:{signal.get('signal_id', 'unknown')}"
                    await rdb.setex(key, 3600, explanation)
                    await producer.send(
                        "signals.explained",
                        json.dumps(signal).encode(),
                    )
            except Exception as exc:
                log.error("LLM service consumer error: %s", type(exc).__name__)

    await consumer.stop()
    await producer.stop()


@app.on_event("startup")
async def startup():
    await get_redis()
    asyncio.create_task(_kafka_consumer_task())
    log.info("LLM service v3.0.0 started (Kimi K2 + Groq narration)")


if __name__ == "__main__":
    uvicorn.run("main:app", host="0.0.0.0", port=8086, reload=False)
