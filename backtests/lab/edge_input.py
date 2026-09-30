
from __future__ import annotations

from pathlib import Path

import pandas as pd

from .dataset import verify_snapshot


def load_edge_search_bars(snapshot_dir: str | Path, *, min_rows: int = 100) -> tuple[pd.DataFrame, dict]:
    """Load only a verified sealed snapshot for downstream edge search."""
    root = Path(snapshot_dir)
    manifest = verify_snapshot(root, min_rows=min_rows)
    bars = pd.read_csv(root / manifest["canonical_file"])
    bars["timestamp"] = pd.to_datetime(bars["timestamp"], utc=True)
    return bars, manifest
