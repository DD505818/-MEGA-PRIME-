import hashlib
import json
import os

import pandas as pd
import pytest

from validation_lab import data


@pytest.fixture()
def venue_dir(tmp_path):
    df = pd.DataFrame({
        "ts": [1000, 1060, 1120],
        "iso8601": ["2023-10-01T00:00:00Z", "2023-10-01T00:01:00Z",
                    "2023-10-01T00:02:00Z"],
        "open": [100.0, 101.0, 102.0],
        "high": [101.0, 102.0, 103.0],
        "low": [99.0, 100.0, 101.0],
        "close": [100.5, 101.5, 102.5],
        "volume": [1.0, 2.0, 3.0],
    })
    p = tmp_path / "venue_X_1m.csv"
    df.to_csv(p, index=False)
    h = hashlib.sha256(p.read_bytes()).hexdigest()
    man = {"venue": "v", "pair": "X",
           "files": {"1m": {"file": p.name, "sha256": h}}}
    (tmp_path / "MANIFEST.json").write_text(json.dumps(man))
    return tmp_path, str(p)


def test_load_ok(venue_dir):
    _, p = venue_dir
    df = data.load_csv(p)
    assert len(df) == 3
    assert df.index.is_monotonic_increasing


def test_hash_mismatch_refuses(venue_dir):
    tmp, p = venue_dir
    with open(p, "ab") as f:
        f.write(b"\n")
    with pytest.raises(data.IntegrityError):
        data.load_csv(p)


def test_missing_manifest(tmp_path):
    p = tmp_path / "x.csv"
    p.write_text("a\n")
    with pytest.raises(data.IntegrityError):
        data.load_csv(str(p))


def test_ohlc_inconsistency_rejected(venue_dir):
    tmp, p = venue_dir
    df = pd.read_csv(p)
    df.loc[0, "low"] = 999.0  # low > high
    df.to_csv(p, index=False)
    # re-sign the manifest so the failure is schema, not hash
    h = hashlib.sha256(open(p, "rb").read()).hexdigest()
    man = json.loads((tmp / "MANIFEST.json").read_text())
    man["files"]["1m"]["sha256"] = h
    (tmp / "MANIFEST.json").write_text(json.dumps(man))
    with pytest.raises(data.IntegrityError):
        data.load_csv(p)
