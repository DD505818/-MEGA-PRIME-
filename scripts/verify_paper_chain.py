#!/usr/bin/env python3
"""Deterministic PAPER lifecycle proof.

This verifies the deployed application path. It is not strategy-performance or
broker-certification evidence.
"""
from __future__ import annotations

import json
import math
import subprocess
import time
import urllib.error
import urllib.request

SIGNAL_ID = "00000000-0000-4000-8000-000000000058"
EXPECTED_FILL_PRICE = 60005.792369384566
SIGNAL = {
    "signal_id": SIGNAL_ID,
    "strategy_id": "PAPER_PROOF",
    "symbol": "BTCUSD",
    "side": "BUY",
    "quantity": 0.001,
    "limit_price": 60000.0,
    "stop": 59000.0,
    "target": 62000.0,
    "confidence": 0.90,
    "mode": "paper",
    "reason": "deterministic deployment proof",
}


def run(*args: str, input_text: str | None = None) -> str:
    result = subprocess.run(
        args,
        input=input_text,
        text=True,
        check=True,
        capture_output=True,
    )
    return result.stdout.strip()


def request_json(url: str, payload: dict | None = None) -> object:
    body = None if payload is None else json.dumps(payload).encode()
    req = urllib.request.Request(
        url,
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST" if body is not None else "GET",
    )
    with urllib.request.urlopen(req, timeout=5) as response:
        return json.load(response)


def wait_json(url: str, predicate, timeout: int = 120):
    deadline = time.monotonic() + timeout
    last_error: Exception | None = None
    while time.monotonic() < deadline:
        try:
            value = request_json(url)
            if predicate(value):
                return value
        except (OSError, urllib.error.URLError, json.JSONDecodeError) as exc:
            last_error = exc
        time.sleep(1)
    raise RuntimeError(f"timed out waiting for {url}: {last_error}")


def audit(event_type: str, payload: object) -> object:
    return request_json(
        "http://127.0.0.1:8084/append",
        {"event_type": event_type, "payload": payload},
    )


def main() -> None:
    for port in (8080, 8081, 8083, 8084):
        wait_json(
            f"http://127.0.0.1:{port}/health/ready",
            lambda value: isinstance(value, dict) and value.get("status") == "ready",
        )

    expected_env = {
        "risk-engine": {
            "PAPER_MODE": "true",
            "LIVE_TRADING_ENABLED": "false",
        },
        "execution-engine": {
            "PAPER_MODE": "true",
            "PAPER_DETERMINISTIC": "true",
            "LIVE_TRADING_ENABLED": "false",
        },
    }
    for service, variables in expected_env.items():
        for key, expected in variables.items():
            actual = run(
                "docker", "compose", "exec", "-T", service,
                "sh", "-c", f'printf %s "${key}"',
            )
            if actual != expected:
                raise RuntimeError(
                    f"{service} is not fail-closed: {key}={actual!r}, expected {expected!r}"
                )

    live_attempt = subprocess.run(
        [
            "docker", "compose", "run", "--rm", "--no-deps", "-T",
            "-e", "PAPER_MODE=false",
            "-e", "LIVE_TRADING_ENABLED=true",
            "execution-engine",
        ],
        text=True,
        capture_output=True,
    )
    if live_attempt.returncode == 0:
        raise RuntimeError("execution service accepted LIVE mode before certification")
    if "LIVE execution is disabled" not in live_attempt.stderr:
        raise RuntimeError(f"unexpected LIVE guard failure: {live_attempt.stderr}")

    now_ms = str(int(time.time() * 1000))
    seeds = {
        "portfolio:equity": "2000",
        "portfolio:peak_equity": "2000",
        "portfolio:daily_pnl": "0",
        "portfolio:open_positions": "0",
        "broker:status": "UP",
        "book_ts:BTCUSD": now_ms,
        "book_spread:BTCUSD": "5",
    }
    for key, value in seeds.items():
        run("docker", "compose", "exec", "-T", "redis", "redis-cli", "SET", key, value)

    audit("paper.market_snapshot", {"symbol": "BTCUSD", "timestamp_ms": int(now_ms)})
    audit("paper.proposal", SIGNAL)

    run(
        "docker", "compose", "exec", "-T", "kafka",
        "kafka-console-producer",
        "--bootstrap-server", "kafka:9092",
        "--topic", "signals.raw",
        input_text=json.dumps(SIGNAL) + "\n",
    )

    orders = wait_json(
        "http://127.0.0.1:8081/orders",
        lambda values: any(
            order.get("signal_id") == SIGNAL_ID and order.get("state") == "FILLED"
            for order in values
        ),
    )
    order = next(order for order in orders if order.get("signal_id") == SIGNAL_ID)
    fill_price = float(order["avg_fill_price"])
    if not math.isclose(fill_price, EXPECTED_FILL_PRICE, rel_tol=0.0, abs_tol=1e-9):
        raise RuntimeError(
            f"non-deterministic PAPER fill: {fill_price} != {EXPECTED_FILL_PRICE}"
        )
    audit("paper.risk_approved", {
        "signal_id": SIGNAL_ID,
        "adjusted_quantity": order["quantity"],
    })
    audit("paper.fill", order)

    positions = wait_json(
        "http://127.0.0.1:8083/positions",
        lambda values: any(
            position.get("symbol") == "BTCUSD" and position.get("quantity", 0) > 0
            for position in values
        ),
    )
    position = next(position for position in positions if position.get("symbol") == "BTCUSD")
    audit("paper.position", position)

    expected = float(order["filled_qty"])
    actual = float(position["quantity"])
    reconciled = math.isclose(expected, actual, rel_tol=0.0, abs_tol=1e-12)
    reconciliation = {
        "signal_id": SIGNAL_ID,
        "order_id": order["order_id"],
        "symbol": "BTCUSD",
        "expected_quantity": expected,
        "actual_quantity": actual,
        "reconciled": reconciled,
        "source": "independent-proof-ledger",
    }
    audit("paper.reconciliation", reconciliation)
    if not reconciled:
        raise RuntimeError(f"reconciliation mismatch: {reconciliation}")

    verification = request_json("http://127.0.0.1:8084/verify")
    if not verification.get("valid") or verification.get("verified_entries", 0) < 6:
        raise RuntimeError(f"TruthCore verification failed: {verification}")

    result = {
        "mode": "PAPER",
        "live_enabled": False,
        "signal_id": SIGNAL_ID,
        "order_id": order["order_id"],
        "fill_price": fill_price,
        "position_quantity": actual,
        "reconciled": True,
        "truthcore": verification,
    }
    print(json.dumps(result, indent=2, sort_keys=True))


if __name__ == "__main__":
    main()
