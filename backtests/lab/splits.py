
from __future__ import annotations

from dataclasses import asdict, dataclass
from typing import Iterable

import pandas as pd


@dataclass(frozen=True)
class Fold:
    fold: int
    train_start_index: int
    train_end_index: int
    test_start_index: int
    test_end_index: int
    train_start: str
    train_end: str
    test_start: str
    test_end: str
    excluded_bars: int


def _iso(value: pd.Timestamp) -> str:
    return value.isoformat().replace("+00:00", "Z")


def purged_walk_forward(
    timestamps: Iterable,
    *,
    train_bars: int,
    test_bars: int,
    purge_bars: int = 0,
    embargo_bars: int = 0,
    step_bars: int | None = None,
    anchored: bool = True,
) -> list[dict]:
    if min(train_bars, test_bars) <= 0:
        raise ValueError("train_bars and test_bars must be positive")
    if purge_bars < 0 or embargo_bars < 0:
        raise ValueError("purge_bars and embargo_bars must be non-negative")
    if train_bars <= purge_bars + embargo_bars:
        raise ValueError("train_bars must exceed purge_bars + embargo_bars")

    ts = pd.DatetimeIndex(pd.to_datetime(list(timestamps), utc=True)).drop_duplicates().sort_values()
    step = step_bars or test_bars
    if step <= 0:
        raise ValueError("step_bars must be positive")

    folds: list[Fold] = []
    test_start = train_bars
    fold_no = 0
    while test_start + test_bars <= len(ts):
        raw_train_start = 0 if anchored else test_start - train_bars
        train_end_exclusive = test_start - purge_bars - embargo_bars
        if train_end_exclusive <= raw_train_start:
            break
        test_end_exclusive = test_start + test_bars

        train_idx = range(raw_train_start, train_end_exclusive)
        test_idx = range(test_start, test_end_exclusive)
        if set(train_idx).intersection(test_idx):
            raise AssertionError("walk-forward split overlap")

        folds.append(
            Fold(
                fold=fold_no,
                train_start_index=raw_train_start,
                train_end_index=train_end_exclusive - 1,
                test_start_index=test_start,
                test_end_index=test_end_exclusive - 1,
                train_start=_iso(ts[raw_train_start]),
                train_end=_iso(ts[train_end_exclusive - 1]),
                test_start=_iso(ts[test_start]),
                test_end=_iso(ts[test_end_exclusive - 1]),
                excluded_bars=purge_bars + embargo_bars,
            )
        )
        fold_no += 1
        test_start += step

    if not folds:
        raise ValueError("not enough timestamps to create a walk-forward fold")
    return [asdict(f) for f in folds]
