"""Promotion acceptance tests: PASS in, anything else rejected."""

import copy

import pytest

from midas import promote
from midas.registry import PROMOTED, REJECTED

from .conftest import DATASET_DIR, make_pass_report


def _promote(registry, strategy, report, thresholds, by="tester"):
    return promote(
        registry,
        strategy,
        report,
        promoted_by=by,
        thresholds=thresholds,
        dataset_dir=DATASET_DIR,
    )


def test_pass_report_promotes(registry, thresholds):
    ok, msg = _promote(registry, "BoxTheory", make_pass_report(), thresholds)
    assert ok, msg
    rec = registry.get("BoxTheory")
    assert rec["status"] == PROMOTED
    assert rec["verdict"] == "PASS"
    assert rec["report_sha256"]
    assert registry.is_promoted("BoxTheory")
    assert rec["metrics"]["net_sharpe"] == 1.8


def test_fail_verdict_rejected(registry, thresholds):
    rep = make_pass_report(verdict="FAIL", failed_stage="costs")
    ok, msg = _promote(registry, "BoxTheory", rep, thresholds)
    assert not ok
    assert registry.get("BoxTheory")["status"] == REJECTED
    assert not registry.is_promoted("BoxTheory")
    assert "not PASS" in msg


def test_tampered_stage_rejected(registry, thresholds):
    """verdict claims PASS but a stage does not -- rejected, reason recorded."""
    rep = make_pass_report()
    rep["stages"]["costs"]["status"] = "FAIL"
    ok, msg = _promote(registry, "Surge", rep, thresholds)
    assert not ok
    assert "costs" in msg
    assert registry.get("Surge")["status"] == REJECTED


def test_unknown_data_hash_rejected(registry, thresholds):
    rep = make_pass_report()
    rep["data"] = dict(rep["data"], sha256="00" * 32)
    ok, msg = _promote(registry, "BoxTheory", rep, thresholds)
    assert not ok
    assert "manifest" in msg


def test_weaker_report_bar_rejected(registry, thresholds):
    """A report generated with a weaker bar than MIDAS's is rejected."""
    rep = make_pass_report()
    rep["config"]["min_oos_sharpe"] = 0.1  # below MIDAS default 0.5
    ok, msg = _promote(registry, "BoxTheory", rep, thresholds)
    assert not ok
    assert "weaker bar" in msg


def test_missing_config_key_rejected(registry, thresholds):
    rep = make_pass_report()
    del rep["config"]["min_dsr"]
    ok, msg = _promote(registry, "BoxTheory", rep, thresholds)
    assert not ok
    assert "min_dsr" in msg


def test_unreadable_report_rejected(registry, thresholds):
    ok, msg = _promote(registry, "BoxTheory", "/nonexistent/report.json", thresholds)
    assert not ok
    assert registry.get("BoxTheory")["status"] == REJECTED


def test_unknown_strategy_not_promoted(registry):
    assert not registry.is_promoted("NoSuchStrategy")
    assert registry.get("NoSuchStrategy") is None


def test_idempotent_repromote_same_report(registry, thresholds):
    rep = make_pass_report()
    ok, _ = _promote(registry, "BoxTheory", rep, thresholds)
    assert ok
    ok2, msg2 = _promote(registry, "BoxTheory", rep, thresholds)
    assert ok2 and "already PROMOTED" in msg2


def test_fresh_report_repromotes_after_reject(registry, thresholds):
    """REJECTED is not terminal: a new evaluation may succeed."""
    bad = make_pass_report(verdict="FAIL", failed_stage="costs")
    ok, _ = _promote(registry, "BoxTheory", bad, thresholds)
    assert not ok
    ok2, _ = _promote(registry, "BoxTheory", make_pass_report(), thresholds)
    assert ok2
    assert registry.is_promoted("BoxTheory")


def test_stricter_report_bar_accepted(registry, thresholds):
    rep = make_pass_report()
    rep["config"]["min_oos_sharpe"] = 1.0  # stricter than MIDAS bar: fine
    ok, msg = _promote(registry, "BoxTheory", rep, thresholds)
    assert ok, msg
