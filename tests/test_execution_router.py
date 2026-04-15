import pytest

from services.execution_router.app.logic import select_exchange


def test_exchange_selection_prefers_best_rank():
    candidates = [
        {"name": "A", "spread": 1.5, "liquidity": 100, "latency_ms": 20},
        {"name": "B", "spread": 1.1, "liquidity": 80, "latency_ms": 15},
    ]
    assert select_exchange(candidates)["name"] == "B"


def test_exchange_selection_rejects_empty_candidates():
    with pytest.raises(ValueError, match="at least one venue quote"):
        select_exchange([])


def test_exchange_selection_rejects_incomplete_quote():
    candidates = [{"name": "A", "spread": 0.8, "latency_ms": 12}]
    with pytest.raises(ValueError, match="missing required metrics"):
        select_exchange(candidates)


def test_exchange_selection_rejects_non_mapping_candidates():
    with pytest.raises(TypeError, match="candidate must be a mapping"):
        select_exchange(["kraken"])


def test_exchange_selection_rejects_non_numeric_metrics():
    with pytest.raises(ValueError, match="must be numeric"):
        select_exchange([{"spread": "tight", "liquidity": 100, "latency_ms": 12}])


def test_exchange_selection_normalizes_numeric_strings():
    selected = select_exchange(
        [
            {"name": "A", "spread": "1.20", "liquidity": "100", "latency_ms": "20"},
            {"name": "B", "spread": "1.10", "liquidity": "80", "latency_ms": "15"},
        ]
    )
    assert selected["name"] == "B"
