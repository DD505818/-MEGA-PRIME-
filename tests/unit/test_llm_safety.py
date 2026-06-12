"""12 LLM safety tests for the Kimi K2 integration.

Tests run without network access — kimi_service internals are exercised
through unit-level fakes (no real API calls, no real Redis).
"""
from __future__ import annotations

import asyncio
import hashlib
import json
import os
import sys
import types
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

# ── Stub httpx before importing kimi_service ─────────────────────────────────
if "httpx" not in sys.modules:
    httpx_stub = types.ModuleType("httpx")
    httpx_stub.AsyncClient = MagicMock
    httpx_stub.Timeout = MagicMock
    httpx_stub.TransportError = Exception
    httpx_stub.TimeoutException = Exception
    httpx_stub.HTTPStatusError = Exception
    sys.modules["httpx"] = httpx_stub

# Stub redis.asyncio
if "redis" not in sys.modules:
    redis_mod = types.ModuleType("redis")
    redis_async = types.ModuleType("redis.asyncio")
    redis_mod.asyncio = redis_async
    sys.modules["redis"] = redis_mod
    sys.modules["redis.asyncio"] = redis_async

# Stub aiokafka
if "aiokafka" not in sys.modules:
    sys.modules["aiokafka"] = types.ModuleType("aiokafka")

# Add llm-service to path
LLM_PATH = str((
    __import__("pathlib").Path(__file__).resolve().parents[2]
    / "apps" / "llm-service"
))
if LLM_PATH not in sys.path:
    sys.path.insert(0, LLM_PATH)

import kimi_service as ks  # noqa: E402


# ── Helpers ───────────────────────────────────────────────────────────────────
def _fake_redis(*, kill=False, spend=0.0):
    rdb = AsyncMock()

    async def _get(key):
        if "kill" in key:
            return b"1" if kill else None
        if "spend" in key:
            return str(spend).encode()
        if "head" in key:
            return b"genesis"
        return None

    rdb.get.side_effect = _get

    pipe = MagicMock()
    pipe.rpush = MagicMock()
    pipe.set = MagicMock()
    pipe.incrbyfloat = MagicMock()
    pipe.expire = MagicMock()
    pipe.execute = AsyncMock(return_value=[1, True])
    # pipeline() is synchronous in redis.asyncio — override with MagicMock
    rdb.pipeline = MagicMock(return_value=pipe)
    return rdb


# ── Test 1: API key never appears in log output ───────────────────────────────
def test_api_key_never_logged(caplog):
    fake_key = "sk-live-supersecret-key-abc123"
    with patch.dict(os.environ, {"KIMI_API_KEY": fake_key}):
        import importlib
        importlib.reload(ks)
        assert ks._KIMI_API_KEY == fake_key
    redacted = ks._redact_key(f"Authorization: Bearer {fake_key}")
    assert fake_key not in redacted
    assert "[REDACTED]" in redacted


# ── Test 2: _redact_key handles empty key without crash ──────────────────────
def test_redact_key_empty_key_safe():
    with patch.object(ks, "_KIMI_API_KEY", ""):
        result = ks._redact_key("some log line with text")
    assert result == "some log line with text"


# ── Test 3: Kill switch blocks stream_chat ───────────────────────────────────
@pytest.mark.asyncio
async def test_kill_switch_blocks_chat():
    rdb = _fake_redis(kill=True)
    with pytest.raises(PermissionError, match="Kill switch"):
        async for _ in ks.stream_chat(rdb, "user1", [{"role": "user", "content": "hi"}]):
            pass


# ── Test 4: Missing API key raises RuntimeError ──────────────────────────────
@pytest.mark.asyncio
async def test_missing_api_key_raises():
    rdb = _fake_redis(kill=False, spend=0.0)
    with patch.object(ks, "_KIMI_API_KEY", ""):
        with pytest.raises(RuntimeError, match="KIMI_API_KEY"):
            async for _ in ks.stream_chat(rdb, "user1", [{"role": "user", "content": "hi"}]):
                pass


# ── Test 5: Daily spend cap blocks when exceeded ─────────────────────────────
@pytest.mark.asyncio
async def test_daily_spend_cap_enforced():
    rdb = _fake_redis(kill=False, spend=999.99)
    with patch.object(ks, "_KIMI_API_KEY", "sk-test-key"):
        with patch.object(ks, "DAILY_SPEND_CAP_USD", 5.0):
            with pytest.raises(PermissionError, match="Daily LLM spend cap"):
                async for _ in ks.stream_chat(rdb, "user1", [{"role": "user", "content": "hi"}]):
                    pass


# ── Test 6: Spend cap allows when within budget ───────────────────────────────
@pytest.mark.asyncio
async def test_spend_cap_allows_within_budget():
    rdb = _fake_redis(kill=False, spend=0.01)
    with patch.object(ks, "DAILY_SPEND_CAP_USD", 5.0):
        within, spent = await ks.check_spend_cap(rdb, "user1")
    assert within is True
    assert spent == 0.01


# ── Test 7: check_kill_switch returns True when active ───────────────────────
@pytest.mark.asyncio
async def test_check_kill_switch_active():
    rdb = AsyncMock()
    rdb.get.return_value = b"1"
    result = await ks.check_kill_switch(rdb)
    assert result is True


# ── Test 8: check_kill_switch returns False when inactive ────────────────────
@pytest.mark.asyncio
async def test_check_kill_switch_inactive():
    rdb = AsyncMock()
    rdb.get.return_value = None
    result = await ks.check_kill_switch(rdb)
    assert result is False


# ── Test 9: System prompt refuses buy/sell signals ────────────────────────────
def test_system_prompt_refuses_signals():
    prompt = ks.SYSTEM_PROMPT
    refusal_terms = [
        "never issue",
        "not issue buy or sell",
        "does not issue",
    ]
    lower = prompt.lower()
    found = any(term in lower for term in refusal_terms)
    assert found, "System prompt must explicitly refuse to issue buy/sell signals"


# ── Test 10: System prompt requires uncertainty language ─────────────────────
def test_system_prompt_requires_uncertainty():
    prompt = ks.SYSTEM_PROMPT
    uncertainty_terms = ["uncertainty", "probabilistic", "historically", "tends to", "may indicate"]
    lower = prompt.lower()
    found = any(term in lower for term in uncertainty_terms)
    assert found, "System prompt must require uncertainty language"


# ── Test 11: Audit hash-chain is deterministic and advances ──────────────────
@pytest.mark.asyncio
async def test_audit_chain_advances():
    captured: list[str] = []

    rdb = AsyncMock()
    rdb.get.return_value = b"genesis_hash_abc"

    pipe = MagicMock()
    pipe.rpush = MagicMock(side_effect=lambda k, v: captured.append(v))
    pipe.set = MagicMock()
    pipe.execute = AsyncMock(return_value=[1, True])
    rdb.pipeline = MagicMock(return_value=pipe)

    await ks._chain_audit(rdb, "user1", 50, 100, 0.0003)
    assert len(captured) == 1
    entry = json.loads(captured[0])
    assert entry["event"] == "llm_chat"
    assert entry["prev_hash"] == "genesis_hash_abc"
    # Verify the hash is deterministic SHA-256
    raw = json.dumps(
        {k: v for k, v in entry.items() if k != "hash"}, sort_keys=True
    )
    expected_hash = hashlib.sha256(("genesis_hash_abc" + raw).encode()).hexdigest()
    assert entry["hash"] == expected_hash


# ── Test 12: Cost calculation is never negative ───────────────────────────────
def test_cost_calculation_non_negative():
    for prompt_t, comp_t in [(0, 0), (1, 0), (0, 1), (1000, 500), (100_000, 50_000)]:
        cost = ks._cost_usd(prompt_t, comp_t)
        assert cost >= 0.0, f"Cost must be non-negative for ({prompt_t}, {comp_t})"
    # Sanity check: 1M input + 1M output tokens
    cost_large = ks._cost_usd(1_000_000, 1_000_000)
    assert cost_large == pytest.approx(ks._COST_INPUT_PER_M + ks._COST_OUTPUT_PER_M)
