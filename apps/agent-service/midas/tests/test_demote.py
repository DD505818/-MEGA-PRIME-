"""Demotion tests: PAPER outcomes breach the bar -> DEMOTED, fresh report to return."""

import pytest

from midas import demote as demote_mod
from midas import promote
from midas.registry import DEMOTED, PROMOTED

from .conftest import DATASET_DIR, make_pass_report


def _promoted(registry, thresholds, strategy="BoxTheory", report=None):
    ok, msg = promote(
        registry,
        strategy,
        report or make_pass_report(),
        promoted_by="tester",
        thresholds=thresholds,
        dataset_dir=DATASET_DIR,
    )
    assert ok, msg
    return strategy


def test_demote_on_sharpe_floor_breach(registry, thresholds):
    s = _promoted(registry, thresholds)
    demoted_any = False
    for _ in range(30):
        demoted, msg = demote_mod.record_fill(registry, s, -1.0, thresholds=thresholds)
        demoted_any = demoted_any or demoted
    assert demoted_any, "30 losing fills must breach the Sharpe floor"
    assert registry.get(s)["status"] == DEMOTED
    assert not registry.is_promoted(s)


def test_no_demote_below_min_fills(registry, thresholds):
    s = _promoted(registry, thresholds)
    for _ in range(29):  # one short of the 30-fill minimum
        demoted, msg = demote_mod.record_fill(registry, s, -100.0, thresholds=thresholds)
        assert not demoted
    assert registry.get(s)["status"] == PROMOTED


def test_no_demote_when_healthy(registry, thresholds):
    s = _promoted(registry, thresholds)
    for i in range(30):
        demoted, _ = demote_mod.record_fill(
            registry, s, 1.0 + (i % 3), thresholds=thresholds
        )
        assert not demoted
    assert registry.get(s)["status"] == PROMOTED


def test_demote_on_drawdown_breach(registry, thresholds):
    s = _promoted(registry, thresholds)
    # build a peak, then one loss > 25% of peak with Sharpe still positive
    for _ in range(30):
        demote_mod.record_fill(registry, s, 10.0, thresholds=thresholds)
    demoted, msg = demote_mod.record_fill(registry, s, -100.0, thresholds=thresholds)
    assert demoted, msg
    assert "drawdown" in msg
    assert registry.get(s)["status"] == DEMOTED


def test_repromote_requires_fresh_report(registry, thresholds):
    s = _promoted(registry, thresholds)
    for _ in range(30):
        demote_mod.record_fill(registry, s, -5.0, thresholds=thresholds)
    assert registry.get(s)["status"] == DEMOTED

    # same report bytes -> rejected, never reused
    ok, msg = promote(
        registry, s, make_pass_report(), promoted_by="tester",
        thresholds=thresholds, dataset_dir=DATASET_DIR,
    )
    assert not ok
    assert "fresh PASS report" in msg
    assert registry.get(s)["status"] == DEMOTED  # still demoted (rejected eval)

    # a genuinely new report (different bytes) -> promoted again
    fresh = make_pass_report()
    fresh["stages"]["costs"]["metrics"]["net_sharpe"] = 2.4
    ok2, msg2 = promote(
        registry, s, fresh, promoted_by="tester",
        thresholds=thresholds, dataset_dir=DATASET_DIR,
    )
    assert ok2, msg2
    assert registry.is_promoted(s)
    # demotion history survived the round-trip
    assert len(registry.get(s)["demotion_history"]) == 1


def test_demotion_not_applicable_when_not_promoted(registry, thresholds):
    demoted, msg = demote_mod.evaluate(registry, "Ghost", thresholds=thresholds)
    assert not demoted
    assert "not PROMOTED" in msg


def test_manual_demote(registry, thresholds):
    s = _promoted(registry, thresholds)
    ok, msg = demote_mod.manual_demote(registry, s, "operator review")
    assert ok
    assert registry.get(s)["status"] == DEMOTED


def test_trailing_stats_edges():
    st = demote_mod.trailing_stats([])
    assert st["n"] == 0
    st = demote_mod.trailing_stats([1.0] * 10)
    assert st["sharpe"] > 0 and st["max_drawdown"] == 0.0
