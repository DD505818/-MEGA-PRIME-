"""Gate tests: the AGENTS -> MIDAS enforcement point (no Kafka involved)."""

import pytest

from midas import is_promoted, promote, reset_default_registry
from midas.registry import PromotionRegistry

from .conftest import DATASET_DIR, make_pass_report


def _gate_filter(names, registry):
    """Mirrors the orchestrator publish-loop gate."""
    return [n for n in names if is_promoted(n, registry)]


def test_gate_blocks_unpromoted(registry, thresholds):
    ok, _ = promote(
        registry, "BoxTheory", make_pass_report(), promoted_by="t",
        thresholds=thresholds, dataset_dir=DATASET_DIR,
    )
    assert ok
    allowed = _gate_filter(["BoxTheory", "Surge", "Ghost"], registry)
    assert allowed == ["BoxTheory"]


def test_gate_fail_closed_on_broken_registry():
    class Broken:
        def is_promoted(self, _):
            raise RuntimeError("db gone")

    assert is_promoted("BoxTheory", Broken()) is False


def test_gate_fail_closed_on_none_registry(tmp_path, monkeypatch):
    monkeypatch.setenv("MIDAS_DB_PATH", str(tmp_path / "x.db"))
    reset_default_registry()
    try:
        # fresh default registry: nothing promoted
        assert is_promoted("BoxTheory") is False
    finally:
        reset_default_registry()
