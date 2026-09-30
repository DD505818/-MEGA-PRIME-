"""PAPER/LIVE lock — canonical mode check for Python services.

Mirrors apps/modelock (Go): the only runnable mode is paper. LIVE is locked.
Fail closed: anything but an explicit PAPER_MODE=true refuses to start, and
LIVE_TRADING_ENABLED=true vetoes unconditionally.
"""
from __future__ import annotations

import os
import sys


def _is_true(v: str | None) -> bool:
    return (v or "").strip().lower() == "true"


def is_paper() -> bool:
    if _is_true(os.getenv("LIVE_TRADING_ENABLED")):
        return False
    return _is_true(os.getenv("PAPER_MODE"))


def require_paper(service: str) -> None:
    """Exit non-zero unless in paper mode. Call once at startup."""
    if not is_paper():
        print(
            f'modelock: {service} refusing to start: PAPER_MODE must be "true" '
            'and LIVE_TRADING_ENABLED must not be "true" (LIVE is locked)',
            file=sys.stderr,
        )
        sys.exit(1)
    print(f"modelock: {service} started in PAPER mode (LIVE locked)")
