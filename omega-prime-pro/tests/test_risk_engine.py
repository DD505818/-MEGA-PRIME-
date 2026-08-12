import importlib.util
from pathlib import Path

SERVICE_ROOT = Path(__file__).resolve().parents[1] / "services"
module_path = SERVICE_ROOT / "risk-engine" / "risk_guard.py"
spec = importlib.util.spec_from_file_location("risk_guard", module_path)
assert spec is not None and spec.loader is not None
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def test_should_halt_on_drawdown():
    assert module.should_halt(0.0, 0.11, 0.1)
