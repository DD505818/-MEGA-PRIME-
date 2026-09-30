
from __future__ import annotations

import hashlib
import json
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

import numpy as np
import pandas as pd

BARS_SCHEMA_VERSION = "omega-bars-v1"
TICKS_SCHEMA_VERSION = "omega-ticks-v1"
BARS_COLUMNS = ("timestamp", "symbol", "open", "high", "low", "close", "volume")
TICK_COLUMNS = ("timestamp", "exchange", "symbol", "price", "bid", "ask", "volume")
SUPPORTED_SCHEMAS = (BARS_SCHEMA_VERSION, TICKS_SCHEMA_VERSION)


class DataQualityError(ValueError):
    """Raised when a dataset violates a hard research-data invariant."""


@dataclass(frozen=True)
class QualitySummary:
    rows: int
    symbols: list[str]
    exchanges: list[str]
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


def detect_schema(frame: pd.DataFrame) -> str:
    cols = set(frame.columns)
    if set(TICK_COLUMNS).issubset(cols):
        return TICKS_SCHEMA_VERSION
    if set(BARS_COLUMNS).issubset(cols):
        return BARS_SCHEMA_VERSION
    raise DataQualityError(
        "unable to detect market-data schema; expected OHLCV bars or "
        "exchange/symbol/price/bid/ask/volume ticks"
    )


def _validate_common(
    frame: pd.DataFrame,
    *,
    columns: tuple[str, ...],
    sort_columns: list[str],
    duplicate_columns: list[str],
    min_rows: int,
    allow_reorder: bool,
) -> tuple[pd.DataFrame, bool, int]:
    missing = [c for c in columns if c not in frame.columns]
    if missing:
        raise DataQualityError(f"missing required columns: {missing}")
    if len(frame) < min_rows:
        raise DataQualityError(f"dataset has {len(frame)} rows; minimum is {min_rows}")

    df = frame.loc[:, columns].copy()
    ts = pd.to_datetime(df["timestamp"], utc=True, errors="coerce")
    if ts.isna().any():
        raise DataQualityError(f"{int(ts.isna().sum())} rows have invalid timestamps")
    df["timestamp"] = ts

    for text_col in ("exchange", "symbol"):
        if text_col not in df.columns:
            continue
        df[text_col] = df[text_col].astype(str).str.strip()
        if (df[text_col] == "").any():
            raise DataQualityError(f"empty {text_col} values are not allowed")

    duplicate_keys = int(df.duplicated(duplicate_columns).sum())
    if duplicate_keys:
        raise DataQualityError(f"{duplicate_keys} duplicate {tuple(duplicate_columns)} rows")

    ordered = df.sort_values(sort_columns, kind="mergesort").reset_index(drop=True)
    source_order_changed = not df.reset_index(drop=True).equals(ordered)
    if source_order_changed and not allow_reorder:
        raise DataQualityError(
            f"rows are not ordered by {','.join(sort_columns)}; "
            "use allow_reorder only after source review"
        )
    return ordered, source_order_changed, duplicate_keys


def _gap_summary(df: pd.DataFrame) -> tuple[dict[str, int], dict[str, float | None]]:
    gaps: dict[str, int] = {}
    medians: dict[str, float | None] = {}
    group_cols = ["symbol"]
    if "exchange" in df.columns:
        group_cols = ["exchange", "symbol"]
    for key, group in df.groupby(group_cols, sort=True):
        label = "/".join(key) if isinstance(key, tuple) else str(key)
        diffs = group["timestamp"].diff().dt.total_seconds().dropna()
        positive = diffs[diffs > 0]
        if positive.empty:
            medians[label] = None
            gaps[label] = 0
            continue
        median = float(positive.median())
        medians[label] = median
        gaps[label] = int((positive > median * 10).sum())
    return gaps, medians


def _quality(df: pd.DataFrame, source_order_changed: bool, duplicate_keys: int) -> QualitySummary:
    gaps, medians = _gap_summary(df)
    exchanges = sorted(str(x) for x in df["exchange"].unique()) if "exchange" in df.columns else []
    return QualitySummary(
        rows=int(len(df)),
        symbols=sorted(str(x) for x in df["symbol"].unique()),
        exchanges=exchanges,
        start=_iso_utc(df["timestamp"].min()),
        end=_iso_utc(df["timestamp"].max()),
        source_order_changed=source_order_changed,
        duplicate_keys=duplicate_keys,
        gap_counts=gaps,
        median_interval_seconds=medians,
    )


def validate_bars(
    frame: pd.DataFrame,
    *,
    min_rows: int = 100,
    allow_reorder: bool = False,
) -> tuple[pd.DataFrame, QualitySummary]:
    df, changed, duplicates = _validate_common(
        frame,
        columns=BARS_COLUMNS,
        sort_columns=["timestamp", "symbol"],
        duplicate_columns=["symbol", "timestamp"],
        min_rows=min_rows,
        allow_reorder=allow_reorder,
    )
    for column in ("open", "high", "low", "close", "volume"):
        df[column] = pd.to_numeric(df[column], errors="coerce")
    numeric = df[["open", "high", "low", "close", "volume"]].to_numpy(dtype=float)
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
    return df, _quality(df, changed, duplicates)


def validate_ticks(
    frame: pd.DataFrame,
    *,
    min_rows: int = 100,
    allow_reorder: bool = False,
) -> tuple[pd.DataFrame, QualitySummary]:
    df, changed, duplicates = _validate_common(
        frame,
        columns=TICK_COLUMNS,
        sort_columns=["timestamp", "exchange", "symbol"],
        duplicate_columns=["exchange", "symbol", "timestamp"],
        min_rows=min_rows,
        allow_reorder=allow_reorder,
    )
    for column in ("price", "bid", "ask", "volume"):
        df[column] = pd.to_numeric(df[column], errors="coerce")
    numeric = df[["price", "bid", "ask", "volume"]].to_numpy(dtype=float)
    if not np.isfinite(numeric).all():
        raise DataQualityError("numeric columns contain NaN or infinite values")
    if (df[["price", "bid", "ask"]] <= 0).any().any():
        raise DataQualityError("tick price/bid/ask must be strictly positive")
    if (df["volume"] < 0).any():
        raise DataQualityError("volume must be non-negative")
    crossed = df["ask"] < df["bid"]
    if crossed.any():
        raise DataQualityError(f"{int(crossed.sum())} rows have ask < bid")
    return df, _quality(df, changed, duplicates)


def validate_market_data(
    frame: pd.DataFrame,
    *,
    schema_version: str | None = None,
    min_rows: int = 100,
    allow_reorder: bool = False,
) -> tuple[pd.DataFrame, QualitySummary, str]:
    schema = schema_version or detect_schema(frame)
    if schema == BARS_SCHEMA_VERSION:
        clean, quality = validate_bars(frame, min_rows=min_rows, allow_reorder=allow_reorder)
    elif schema == TICKS_SCHEMA_VERSION:
        clean, quality = validate_ticks(frame, min_rows=min_rows, allow_reorder=allow_reorder)
    else:
        raise DataQualityError(f"unsupported schema_version: {schema!r}")
    return clean, quality, schema


def canonical_csv_bytes(frame: pd.DataFrame, schema_version: str) -> bytes:
    columns = BARS_COLUMNS if schema_version == BARS_SCHEMA_VERSION else TICK_COLUMNS
    out = frame.copy()
    out["timestamp"] = out["timestamp"].map(_iso_utc)
    return out.to_csv(
        index=False,
        columns=columns,
        lineterminator="\n",
        float_format="%.12g",
    ).encode("utf-8")


def seal_snapshot(
    input_path: str | Path,
    output_dir: str | Path,
    *,
    source_name: str,
    schema_version: str | None = None,
    min_rows: int = 100,
    allow_reorder: bool = False,
) -> dict[str, Any]:
    source_path = Path(input_path)
    out_dir = Path(output_dir)
    out_dir.mkdir(parents=True, exist_ok=True)

    raw_bytes = source_path.read_bytes()
    clean, quality, schema = validate_market_data(
        pd.read_csv(source_path),
        schema_version=schema_version,
        min_rows=min_rows,
        allow_reorder=allow_reorder,
    )
    canonical = canonical_csv_bytes(clean, schema)
    (out_dir / "market-data.csv").write_bytes(canonical)

    source_hash = _sha256_bytes(raw_bytes)
    canonical_hash = _sha256_bytes(canonical)
    manifest_seed = f"{schema}:{source_hash}:{canonical_hash}:{source_name}".encode()
    manifest = {
        "schema_version": schema,
        "manifest_id": _sha256_bytes(manifest_seed),
        "source_name": source_name,
        "source_file": source_path.name,
        "source_sha256": source_hash,
        "canonical_file": "market-data.csv",
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
    schema = manifest.get("schema_version")
    if schema not in SUPPORTED_SCHEMAS:
        raise DataQualityError(f"unsupported schema_version: {schema!r}")
    if manifest.get("quality_gate") != "PASS" or manifest.get("edge_search_ready") is not True:
        raise DataQualityError("snapshot manifest is not edge-search ready")

    data_path = root / manifest.get("canonical_file", "market-data.csv")
    if not data_path.exists():
        raise DataQualityError("canonical market-data file is missing")
    actual_hash = sha256_file(data_path)
    if actual_hash != manifest.get("canonical_sha256"):
        raise DataQualityError("canonical market-data hash does not match manifest")

    clean, quality, _ = validate_market_data(
        pd.read_csv(data_path),
        schema_version=schema,
        min_rows=min_rows,
        allow_reorder=False,
    )
    if _sha256_bytes(canonical_csv_bytes(clean, schema)) != actual_hash:
        raise DataQualityError("canonical serialization is not stable")
    if quality.rows != int(manifest["quality"]["rows"]):
        raise DataQualityError("row count does not match manifest")
    if quality.symbols != list(manifest["quality"]["symbols"]):
        raise DataQualityError("symbol universe does not match manifest")
    if quality.exchanges != list(manifest["quality"].get("exchanges", [])):
        raise DataQualityError("exchange universe does not match manifest")
    return manifest
