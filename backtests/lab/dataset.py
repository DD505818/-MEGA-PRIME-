
from __future__ import annotations

import hashlib
import json
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

SCHEMA_VERSION = "omega-bars-v1"
REQUIRED_COLUMNS = ("timestamp", "symbol", "open", "high", "low", "close", "volume")
NUMERIC_COLUMNS = ("open", "high", "low", "close", "volume")


class DataQualityError(ValueError):
    """Raised when a dataset violates a hard research-data invariant."""


@dataclass(frozen=True)
class QualitySummary:
    rows: int
    symbols: list[str]
    start: str
    end: str
    source_order_changed: bool
    duplicate_keys: int
    gap_counts: dict[str, int]
    median_interval_seconds: dict[str, float | None]


def _sha256_bytes(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def sha256_file(path: str | Path) -> str:
    h = hashlib.sha256()
    with Path(path).open("rb") as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def _iso_utc(value: pd.Timestamp) -> str:
    return value.isoformat().replace("+00:00", "Z")


def validate_bars(
    frame: pd.DataFrame,
    *,
    min_rows: int = 100,
    allow_reorder: bool = False,
) -> tuple[pd.DataFrame, QualitySummary]:
    missing = [c for c in REQUIRED_COLUMNS if c not in frame.columns]
    if missing:
        raise DataQualityError(f"missing required columns: {missing}")
    if len(frame) < min_rows:
        raise DataQualityError(f"dataset has {len(frame)} rows; minimum is {min_rows}")

    df = frame.loc[:, REQUIRED_COLUMNS].copy()
    parsed_ts = pd.to_datetime(df["timestamp"], utc=True, errors="coerce")
    if parsed_ts.isna().any():
        raise DataQualityError(f"{int(parsed_ts.isna().sum())} rows have invalid timestamps")
    df["timestamp"] = parsed_ts

    df["symbol"] = df["symbol"].astype(str).str.strip()
    if (df["symbol"] == "").any():
        raise DataQualityError("empty symbol values are not allowed")

    for column in NUMERIC_COLUMNS:
        df[column] = pd.to_numeric(df[column], errors="coerce")
    numeric = df.loc[:, NUMERIC_COLUMNS].to_numpy(dtype=float)
    if not np.isfinite(numeric).all():
        raise DataQualityError("numeric columns contain NaN or infinite values")
    if (df[["open", "high", "low", "close"]] <= 0).any().any():
        raise DataQualityError("OHLC prices must be strictly positive")
    if (df["volume"] < 0).any():
        raise DataQualityError("volume must be non-negative")

    bad_high = df["high"] < df[["open", "low", "close"]].max(axis=1)
    bad_low = df["low"] > df[["open", "high", "close"]].min(axis=1)
    if bad_high.any() or bad_low.any():
        raise DataQualityError(f"{int((bad_high | bad_low).sum())} rows violate OHLC envelope constraints")

    duplicate_keys = int(df.duplicated(["symbol", "timestamp"]).sum())
    if duplicate_keys:
        raise DataQualityError(f"{duplicate_keys} duplicate (symbol, timestamp) rows")

    source_sorted = df.sort_values(["timestamp", "symbol"], kind="mergesort").reset_index(drop=True)
    original_keys = list(zip(df["symbol"].tolist(), df["timestamp"].astype("int64").tolist()))
    sorted_keys = list(zip(source_sorted["symbol"].tolist(), source_sorted["timestamp"].astype("int64").tolist()))
    source_order_changed = original_keys != sorted_keys
    if source_order_changed and not allow_reorder:
        raise DataQualityError("rows are not ordered by timestamp,symbol; use allow_reorder only after source review")

    gaps: dict[str, int] = {}
    medians: dict[str, float | None] = {}
    for symbol, group in source_sorted.groupby("symbol", sort=True):
        diffs = group["timestamp"].diff().dt.total_seconds().dropna()
        positive = diffs[diffs > 0]
        if positive.empty:
            medians[str(symbol)] = None
            gaps[str(symbol)] = 0
            continue
        median = float(positive.median())
        medians[str(symbol)] = median
        gaps[str(symbol)] = int((positive > median * 10).sum())

    summary = QualitySummary(
        rows=int(len(source_sorted)),
        symbols=[str(x) for x in sorted(source_sorted["symbol"].unique())],
        start=_iso_utc(source_sorted["timestamp"].min()),
        end=_iso_utc(source_sorted["timestamp"].max()),
        source_order_changed=source_order_changed,
        duplicate_keys=duplicate_keys,
        gap_counts=gaps,
        median_interval_seconds=medians,
    )
    return source_sorted, summary


def canonical_csv_bytes(frame: pd.DataFrame) -> bytes:
    out = frame.copy()
    out["timestamp"] = out["timestamp"].map(_iso_utc)
    return out.to_csv(
        index=False,
        columns=REQUIRED_COLUMNS,
        lineterminator="\n",
        float_format="%.12g",
    ).encode("utf-8")


def seal_snapshot(
    input_path: str | Path,
    output_dir: str | Path,
    *,
    source_name: str,
    min_rows: int = 100,
    allow_reorder: bool = False,
) -> dict[str, Any]:
    source_path = Path(input_path)
    out_dir = Path(output_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    raw_bytes = source_path.read_bytes()
    clean, quality = validate_bars(
        pd.read_csv(source_path),
        min_rows=min_rows,
        allow_reorder=allow_reorder,
    )
    canonical = canonical_csv_bytes(clean)
    (out_dir / "bars.csv").write_bytes(canonical)

    source_hash = _sha256_bytes(raw_bytes)
    canonical_hash = _sha256_bytes(canonical)
    manifest_seed = f"{SCHEMA_VERSION}:{source_hash}:{canonical_hash}:{source_name}".encode()
    manifest = {
        "schema_version": SCHEMA_VERSION,
        "manifest_id": _sha256_bytes(manifest_seed),
        "source_name": source_name,
        "source_file": source_path.name,
        "source_sha256": source_hash,
        "canonical_file": "bars.csv",
        "canonical_sha256": canonical_hash,
        "quality_gate": "PASS",
        "edge_search_ready": True,
        "quality": asdict(quality),
    }
    (out_dir / "manifest.json").write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
    )
    return manifest


def verify_snapshot(snapshot_dir: str | Path, *, min_rows: int = 100) -> dict[str, Any]:
    root = Path(snapshot_dir)
    manifest_path = root / "manifest.json"
    if not manifest_path.exists():
        raise DataQualityError("manifest.json is missing")
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    if manifest.get("schema_version") != SCHEMA_VERSION:
        raise DataQualityError(f"unsupported schema_version: {manifest.get('schema_version')!r}")
    if manifest.get("quality_gate") != "PASS" or manifest.get("edge_search_ready") is not True:
        raise DataQualityError("snapshot manifest is not edge-search ready")

    bars_path = root / manifest.get("canonical_file", "bars.csv")
    if not bars_path.exists():
        raise DataQualityError("canonical bars file is missing")
    actual_hash = sha256_file(bars_path)
    if actual_hash != manifest.get("canonical_sha256"):
        raise DataQualityError("canonical bars hash does not match manifest")

    clean, quality = validate_bars(pd.read_csv(bars_path), min_rows=min_rows, allow_reorder=False)
    if _sha256_bytes(canonical_csv_bytes(clean)) != actual_hash:
        raise DataQualityError("canonical serialization is not stable")
    if quality.rows != int(manifest["quality"]["rows"]):
        raise DataQualityError("row count does not match manifest")
    if quality.symbols != list(manifest["quality"]["symbols"]):
        raise DataQualityError("symbol universe does not match manifest")
    return manifest
