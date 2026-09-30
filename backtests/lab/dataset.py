"""Compatibility shim. Canonical implementation: `validation_lab.dataset`."""

from validation_lab.dataset import (
    BARS_COLUMNS,
    BARS_SCHEMA_VERSION,
    SUPPORTED_SCHEMAS,
    TICKS_SCHEMA_VERSION,
    TICK_COLUMNS,
    DataQualityError,
    QualitySummary,
    canonical_csv_bytes,
    detect_schema,
    seal_snapshot,
    sha256_file,
    validate_bars,
    validate_market_data,
    validate_ticks,
    verify_snapshot,
)

__all__ = [
    "BARS_COLUMNS",
    "BARS_SCHEMA_VERSION",
    "SUPPORTED_SCHEMAS",
    "TICKS_SCHEMA_VERSION",
    "TICK_COLUMNS",
    "DataQualityError",
    "QualitySummary",
    "canonical_csv_bytes",
    "detect_schema",
    "seal_snapshot",
    "sha256_file",
    "validate_bars",
    "validate_market_data",
    "validate_ticks",
    "verify_snapshot",
]
