import pytest

from services.risk_engine.app.logic import RISK_LIMITS, should_halt


def test_risk_limits_values():
    assert RISK_LIMITS["risk_per_trade"] == 0.005
    assert RISK_LIMITS["daily_loss_limit"] == 0.02


def test_halt_triggered():
    assert should_halt({"loss_pct": 0.03, "drawdown": 0.0, "exposure": 0.1})


def test_halt_not_triggered_with_safe_state():
    assert not should_halt({"loss_pct": 0.01, "drawdown": 0.05, "exposure": 0.10})


def test_should_halt_requires_state():
    with pytest.raises(ValueError, match="state is required"):
        should_halt(None)


def test_should_halt_rejects_non_mapping_state():
    with pytest.raises(TypeError, match="state must be a mapping"):
        should_halt(["not", "a", "dict"])


def test_should_halt_rejects_non_numeric_metrics():
    with pytest.raises(ValueError, match="must be numeric"):
        should_halt({"loss_pct": "bad"})
