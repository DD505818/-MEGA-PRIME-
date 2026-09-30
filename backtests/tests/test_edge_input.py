
from pathlib import Path

import pandas as pd

from backtests.lab.dataset import seal_snapshot
from backtests.lab.edge_input import load_edge_search_bars


def test_edge_loader_requires_verified_snapshot(tmp_path: Path):
    ts = pd.date_range("2026-01-01", periods=100, freq="min", tz="UTC")
    base = pd.Series(range(100), dtype=float) * 0.01 + 100
    raw = tmp_path / "raw.csv"
    pd.DataFrame(
        {
            "timestamp": ts,
            "symbol": "BTC/USD",
            "open": base,
            "high": base + 1,
            "low": base - 1,
            "close": base + 0.1,
            "volume": 1.0,
        }
    ).to_csv(raw, index=False)
    snap = tmp_path / "snap"
    seal_snapshot(raw, snap, source_name="fixture", min_rows=100)
    bars, manifest = load_edge_search_bars(snap)
    assert len(bars) == 100
    assert manifest["edge_search_ready"] is True
