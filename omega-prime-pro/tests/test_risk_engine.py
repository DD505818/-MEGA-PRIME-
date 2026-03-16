import importlib.util
from pathlib import Path

spec = importlib.util.spec_from_file_location("risk_guard", Path("services/risk-engine/risk_guard.py"))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def test_should_halt_on_drawdown():
    assert module.should_halt(0.0, 0.11, 0.1)
