import importlib.util
from pathlib import Path

SERVICE_ROOT = Path(__file__).resolve().parents[1] / "services"
module_path = SERVICE_ROOT / "portfolio-service" / "allocation.py"
spec = importlib.util.spec_from_file_location("allocation", module_path)
assert spec is not None and spec.loader is not None
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def test_allocate_normalizes_weights():
    stats = [module.StrategyStat("a", 1.0, 0.1, 0.9), module.StrategyStat("b", 0.7, 0.1, 0.8)]
    alloc = module.allocate(stats)
    assert round(sum(alloc.values()), 6) == 1.0
