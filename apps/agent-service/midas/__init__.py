"""MIDAS: the promotion gate between AGENTS and AEGIS.

Authority chain: DATA -> MODELS -> AGENTS -> **MIDAS** -> AEGIS -> VULTURE -> ...

No strategy signal may flow toward AEGIS (risk-service, topic "signals.raw")
unless MIDAS has promoted it on the basis of a validation-lab PASS verdict on
after-cost returns. EDGE NOT PROVEN by default; the gate fails closed.

Public surface:
  PromotionRegistry  durable SQLite state machine
  promote(...)       verify a lab report -> PROMOTED / REJECTED (no overrides)
  evaluate(...)      demotion review of PAPER fills
  record_fill(...)   append a PAPER fill + re-evaluate
  manual_demote(...) operator demotion (safe direction only)
  is_promoted(...)   the orchestrator gate: True only for PROMOTED
"""

from __future__ import annotations

from .demote import evaluate, manual_demote, record_fill, trailing_stats
from .promote import STAGE_ORDER, known_data_hashes, promote
from .registry import (
    CANDIDATE,
    DEMOTED,
    PROMOTED,
    REJECTED,
    VALIDATING,
    PromotionRegistry,
)
from .thresholds import Thresholds

_registry = None


def default_registry() -> PromotionRegistry:
    global _registry
    if _registry is None:
        _registry = PromotionRegistry()
    return _registry


def reset_default_registry() -> None:
    """For tests: drop the cached singleton."""
    global _registry
    if _registry is not None:
        try:
            _registry.close()
        except Exception:
            pass
    _registry = None


def is_promoted(strategy_id: str, registry: PromotionRegistry | None = None) -> bool:
    """The AGENTS -> MIDAS gate. Fail closed: unknown, NULL, error -> False."""
    try:
        reg = registry if registry is not None else default_registry()
        return reg.is_promoted(strategy_id)
    except Exception:
        return False


__all__ = [
    "CANDIDATE",
    "DEMOTED",
    "PROMOTED",
    "REJECTED",
    "VALIDATING",
    "STAGE_ORDER",
    "PromotionRegistry",
    "Thresholds",
    "evaluate",
    "is_promoted",
    "known_data_hashes",
    "manual_demote",
    "promote",
    "record_fill",
    "reset_default_registry",
    "trailing_stats",
]
