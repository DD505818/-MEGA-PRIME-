from services.execution_router.app.logic import select_exchange

def test_exchange_selection_prefers_best_rank():
    candidates = [
        {"name": "A", "spread": 1.5, "liquidity": 100, "latency_ms": 20},
        {"name": "B", "spread": 1.1, "liquidity": 80, "latency_ms": 15},
    ]
    assert select_exchange(candidates)["name"] == "B"

def test_exchange_selection_rejects_empty_candidates():
    try:
        select_exchange([])
        assert False, "Expected ValueError for empty candidate list"
    except ValueError as exc:
        assert "at least one venue quote" in str(exc)

def test_exchange_selection_rejects_incomplete_quote():
    candidates = [{"name": "A", "spread": 0.8, "latency_ms": 12}]
    try:
        select_exchange(candidates)
        assert False, "Expected ValueError for incomplete quote metrics"
    except ValueError as exc:
        assert "missing required metrics" in str(exc)
