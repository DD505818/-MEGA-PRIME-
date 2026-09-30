"""Load frozen datasets with SHA-256 integrity verification.

Refuses to load on hash mismatch: a tampered or partial file must never
silently enter the validation pipeline (fail closed).
"""

import hashlib
import json
import os

import pandas as pd

REQUIRED_COLUMNS = ["ts", "iso8601", "open", "high", "low", "close", "volume"]


class IntegrityError(Exception):
    """Raised when a frozen file fails SHA-256 verification against its manifest."""


def sha256_file(path, chunk=1 << 20):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(chunk), b""):
            h.update(block)
    return h.hexdigest()


def manifest_for(path):
    """Locate the venue MANIFEST.json next to the data file."""
    venue_dir = os.path.dirname(os.path.abspath(path))
    mp = os.path.join(venue_dir, "MANIFEST.json")
    if not os.path.exists(mp):
        raise IntegrityError(f"no MANIFEST.json next to {path}")
    with open(mp) as f:
        return json.load(f)


def expected_sha256(manifest, path):
    base = os.path.basename(path)
    for _gran, info in manifest.get("files", {}).items():
        if info.get("file") == base:
            return info["sha256"]
    raise IntegrityError(f"{base} not listed in MANIFEST.json")


def verify(path, manifest=None):
    """Verify SHA-256 of path against the venue manifest. Returns manifest."""
    manifest = manifest or manifest_for(path)
    want = expected_sha256(manifest, path)
    got = sha256_file(path)
    if got != want:
        raise IntegrityError(
            f"SHA-256 mismatch for {path}: manifest={want[:16]}... actual={got[:16]}..."
        )
    return manifest


def load_csv(path, verify_hash=True):
    """Load a frozen OHLCV CSV into a clean DataFrame (datetime index, ascending).

    Raises IntegrityError on hash mismatch or schema problems.
    """
    if verify_hash:
        verify(path)
    df = pd.read_csv(path)
    missing = [c for c in REQUIRED_COLUMNS if c not in df.columns]
    if missing:
        raise IntegrityError(f"{path}: missing columns {missing}")
    df = df[REQUIRED_COLUMNS].copy()
    df["iso8601"] = pd.to_datetime(df["iso8601"], utc=True)
    df = df.sort_values("iso8601").drop_duplicates("iso8601").set_index("iso8601")
    if df.index.duplicated().any():
        raise IntegrityError(f"{path}: duplicate timestamps after dedup")
    if not df.index.is_monotonic_increasing:
        raise IntegrityError(f"{path}: timestamps not monotonic")
    for c in ["open", "high", "low", "close", "volume"]:
        if not pd.to_numeric(df[c], errors="coerce").notna().all():
            raise IntegrityError(f"{path}: non-numeric/NaN in {c}")
    if ((df["low"] > df["high"]).any() or (df["low"] > df["open"]).any()
            or (df["low"] > df["close"]).any() or (df["open"] > df["high"]).any()
            or (df["close"] > df["high"]).any()):
        raise IntegrityError(f"{path}: OHLC inconsistency")
    return df


def describe(df, manifest, path):
    base = os.path.basename(path)
    info = next(v for v in manifest["files"].values() if v["file"] == base)
    return {
        "file": base,
        "sha256": info["sha256"],
        "rows": int(len(df)),
        "first": df.index[0].strftime("%Y-%m-%dT%H:%M:%SZ"),
        "last": df.index[-1].strftime("%Y-%m-%dT%H:%M:%SZ"),
        "venue": manifest.get("venue"),
        "pair": manifest.get("pair"),
        "repo_commit": manifest.get("repo_commit"),
    }
