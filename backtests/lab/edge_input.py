
from __future__ import annotations

from pathlib import Path

import pandas as pd

from .dataset import verify_snapshot


def load_edge_search_data(snapshot_dir: str | Path, *, min_rows: int = 100) -> tuple[pd.DataFrame, dict]:
    """Load only a verified sealed market-data snapshot for edge search."""
    root = Path(snapshot_dir)
    manifest = verify_snapshot(root, min_rows=min_rows)
    data = pd.read_csv(root / manifest["canonical_file"])
    data["timestamp"] = pd.to_datetime(data["timestamp"], utc=True)
    return data, manifest


def load_edge_search_bars(snapshot_dir: str | Path, *, min_rows: int = 100) -> tuple[pd.DataFrame, dict]:
    """Backward-compatible alias; use load_edge_search_data for tick or bar snapshots."""
    return load_edge_search_data(snapshot_dir, min_rows=min_rows)
