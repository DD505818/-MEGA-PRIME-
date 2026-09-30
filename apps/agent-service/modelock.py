"""Canonical PAPER/LIVE runtime contract for Python services.

The contract mirrors apps/modelock (Go). Ambiguous or contradictory mode
configuration is invalid and must fail closed.
"""
from __future__ import annotations

import os
import sys


def _is_true(value: str | None) -> bool:
    return (value or "").strip().lower() == "true"


def current_mode() -> str:
    paper = _is_true(os.getenv("PAPER_MODE"))
    live = _is_true(os.getenv("LIVE_TRADING_ENABLED"))
    trading = (os.getenv("TRADING_MODE") or "").strip().upper()

    if paper and not live and trading in ("", "PAPER"):
        return "paper"

    if (
        not paper
        and live
        and trading == "LIVE"
        and _is_true(os.getenv("BROKER_CERTIFIED"))
        and _is_true(os.getenv("FAILURE_TESTS_PASSED"))
    ):
        return "live"

    raise RuntimeError(
        "modelock: invalid mode contract "
        f"(PAPER_MODE={os.getenv('PAPER_MODE')!r} "
        f"LIVE_TRADING_ENABLED={os.getenv('LIVE_TRADING_ENABLED')!r} "
        f"TRADING_MODE={os.getenv('TRADING_MODE')!r} "
        f"BROKER_CERTIFIED={os.getenv('BROKER_CERTIFIED')!r} "
        f"FAILURE_TESTS_PASSED={os.getenv('FAILURE_TESTS_PASSED')!r})"
    )


def is_paper() -> bool:
    try:
        return current_mode() == "paper"
    except RuntimeError:
        return False


def is_live() -> bool:
    try:
        return current_mode() == "live"
    except RuntimeError:
        return False


def require_mode(service: str) -> str:
    try:
        mode = current_mode()
    except RuntimeError as exc:
        print(f"modelock: {service} refusing to start: {exc}", file=sys.stderr)
        sys.exit(1)
    print(f"modelock: {service} started in {mode.upper()} mode")
    return mode


def require_paper(service: str) -> None:
    if require_mode(service) != "paper":
        print(f"modelock: {service} is PAPER-only", file=sys.stderr)
        sys.exit(1)
