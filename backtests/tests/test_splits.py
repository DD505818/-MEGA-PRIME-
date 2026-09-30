
import pandas as pd

from backtests.lab.splits import purged_walk_forward


def test_purged_walk_forward_has_strict_gap_and_no_overlap():
    ts = pd.date_range("2026-01-01", periods=100, freq="h", tz="UTC")
    folds = purged_walk_forward(
        ts,
        train_bars=40,
        test_bars=10,
        purge_bars=3,
        embargo_bars=2,
        step_bars=10,
    )
    assert len(folds) == 6
    for fold in folds:
        assert fold["test_start_index"] - fold["train_end_index"] - 1 == 5
        assert fold["train_end_index"] < fold["test_start_index"]
