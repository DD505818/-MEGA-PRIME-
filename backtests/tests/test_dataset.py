
from pathlib import Path

import pandas as pd
import pytest

from backtests.lab.dataset import DataQualityError, seal_snapshot, verify_snapshot


def bars(n=120):
    ts = pd.date_range("2026-01-01", periods=n, freq="min", tz="UTC")
    base = pd.Series(range(n), dtype=float) * 0.1 + 100.0
    return pd.DataFrame(
        {
            "timestamp": ts,
            "symbol": ["BTC/USD"] * n,
            "open": base,
            "high": base + 1,
            "low": base - 1,
            "close": base + 0.2,
            "volume": 10.0,
        }
    )


def test_seal_and_verify_detects_tamper(tmp_path: Path):
    source = tmp_path / "raw.csv"
    bars().to_csv(source, index=False)
    out = tmp_path / "snapshot"
    manifest = seal_snapshot(source, out, source_name="fixture")
    assert manifest["edge_search_ready"] is True
    assert verify_snapshot(out)["manifest_id"] == manifest["manifest_id"]

    with (out / "bars.csv").open("a", encoding="utf-8") as f:
        f.write("\n")
    with pytest.raises(DataQualityError, match="hash"):
        verify_snapshot(out)


def test_duplicate_key_rejected(tmp_path: Path):
    df = bars()
    df.iloc[-1] = df.iloc[-2]
    source = tmp_path / "raw.csv"
    df.to_csv(source, index=False)
    with pytest.raises(DataQualityError, match="duplicate"):
        seal_snapshot(source, tmp_path / "out", source_name="fixture")


def test_ohlc_violation_rejected(tmp_path: Path):
    df = bars()
    df.loc[3, "high"] = df.loc[3, "low"] - 1
    source = tmp_path / "raw.csv"
    df.to_csv(source, index=False)
    with pytest.raises(DataQualityError, match="OHLC"):
        seal_snapshot(source, tmp_path / "out", source_name="fixture")


def test_reorder_is_fail_closed_by_default(tmp_path: Path):
    df = bars()
    df = pd.concat([df.iloc[1:2], df.iloc[:1], df.iloc[2:]], ignore_index=True)
    source = tmp_path / "raw.csv"
    df.to_csv(source, index=False)
    with pytest.raises(DataQualityError, match="ordered"):
        seal_snapshot(source, tmp_path / "out", source_name="fixture")
