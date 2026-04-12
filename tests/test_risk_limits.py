from services.risk_engine.app.logic import RISK_LIMITS, should_halt

def test_risk_limits_values():
    assert RISK_LIMITS["risk_per_trade"] == 0.005
    assert RISK_LIMITS["daily_loss_limit"] == 0.02

def test_halt_triggered():
    assert should_halt({"loss_pct": 0.03, "drawdown": 0.0, "exposure": 0.1})

def test_halt_not_triggered_with_safe_state():
    assert not should_halt({"loss_pct": 0.01, "drawdown": 0.05, "exposure": 0.10})

def test_should_halt_requires_state():
    try:
        should_halt(None)
        assert False, "Expected ValueError when state is None"
    except ValueError as exc:
        assert "state is required" in str(exc)
