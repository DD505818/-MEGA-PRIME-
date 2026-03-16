from services.execution_router.app.logic import select_exchange

def test_exchange_selection_prefers_best_rank():
    candidates = [
        {"name": "A", "spread": 1.5, "liquidity": 100, "latency_ms": 20},
        {"name": "B", "spread": 1.1, "liquidity": 80, "latency_ms": 15},
    ]
    assert select_exchange(candidates)["name"] == "B"
