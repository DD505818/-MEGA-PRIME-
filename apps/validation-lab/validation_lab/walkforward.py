"""Purged cross-validation with embargo (Lopez de Prado, AFML ch. 7).

Each sample i carries a label spanning [i, i + label_horizon). A training
sample is purged when its label interval overlaps any test fold interval, and
an embargo zone after each test fold is dropped from training. This removes
the leakage that makes naive K-fold CV overstate strategy performance on
serially correlated financial data.
"""

import numpy as np


def purged_cv_splits(n, n_folds=5, embargo_pct=0.01, label_horizon=1):
    """Return list of (train_idx, test_idx) with purge + embargo applied.

    n: number of samples. Folds are contiguous blocks. embargo_pct: fraction
    of n dropped from training immediately after each test fold.
    """
    if n_folds < 2:
        raise ValueError("n_folds must be >= 2")
    if n < n_folds:
        raise ValueError("n must be >= n_folds")
    fold_size = n // n_folds
    embargo = max(1, int(n * embargo_pct))
    splits = []
    for k in range(n_folds):
        t0 = k * fold_size
        t1 = (k + 1) * fold_size if k < n_folds - 1 else n
        test = np.arange(t0, t1)
        train_mask = np.ones(n, dtype=bool)
        train_mask[t0:t1] = False
        # purge: sample i overlaps test iff i in [t0 - horizon + 1, t1)
        lo = max(0, t0 - label_horizon + 1)
        train_mask[lo:t1] = False
        # embargo: drop embargo bars after the test fold from training
        train_mask[t1:min(n, t1 + embargo)] = False
        splits.append((np.where(train_mask)[0], test))
    return splits


def _position_change_events(positions):
    """Boolean event series: True where the target position changes.

    Index 0 is an event when the series opens non-flat. Later indices are
    events when position[t] != position[t-1]. Counting events (not turnover
    magnitude) keeps a -1 -> +1 flip as one position change, matching the
    cost module's turnover>0 trade count convention.
    """
    pos = np.asarray(positions, dtype=float)
    pos = np.nan_to_num(pos, nan=0.0, posinf=0.0, neginf=0.0)
    events = np.zeros(pos.size, dtype=bool)
    if pos.size:
        events[0] = pos[0] != 0.0
        events[1:] = pos[1:] != pos[:-1]
    return events


def fold_metrics(returns, splits, periods_per_year, positions=None):
    """Per-fold Sharpe/total-return on the test indices.

    When `positions` is supplied, each fold also reports `n_test_trades`:
    position-change events whose index falls inside that fold's test block.
    Events are computed on the full position series, so a position carried
    into a fold is not fabricated as a new trade at the fold boundary.
    """
    from .metrics import sharpe, total_return

    r = np.asarray(returns, dtype=float)
    events = _position_change_events(positions) if positions is not None else None
    out = []
    for train_idx, test_idx in splits:
        tr = r[test_idx]
        entry = {
            "n_test": int(test_idx.size),
            "n_train": int(train_idx.size),
            "test_sharpe": sharpe(tr, periods_per_year),
            "test_total_return": total_return(tr),
            "train_sharpe": sharpe(r[train_idx], periods_per_year),
        }
        if events is not None:
            entry["n_test_trades"] = int(events[test_idx].sum())
        out.append(entry)
    return out


def summarize(folds):
    """Aggregate fold metrics into gauntlet criteria inputs."""
    test_sr = np.array([f["test_sharpe"] for f in folds])
    out = {
        "n_folds": len(folds),
        "mean_test_sharpe": float(np.mean(test_sr)),
        "std_test_sharpe": float(np.std(test_sr, ddof=1)) if len(test_sr) > 1 else 0.0,
        "min_test_sharpe": float(np.min(test_sr)),
        "positive_fold_frac": float(np.mean(test_sr > 0)),
        "mean_test_total_return": float(np.mean([f["test_total_return"] for f in folds])),
        "folds": folds,
    }
    if folds and all("n_test_trades" in f for f in folds):
        trades = [int(f["n_test_trades"]) for f in folds]
        out["oos_trades"] = int(sum(trades))
        out["oos_trades_per_fold"] = trades
        out["oos_trades_min_per_fold"] = int(min(trades)) if trades else 0
    return out
