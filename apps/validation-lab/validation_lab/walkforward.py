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


def fold_metrics(returns, splits, periods_per_year):
    """Per-fold Sharpe/total-return on the test indices. Pure diagnostic."""
    from .metrics import sharpe, total_return

    r = np.asarray(returns, dtype=float)
    out = []
    for train_idx, test_idx in splits:
        tr = r[test_idx]
        out.append({
            "n_test": int(test_idx.size),
            "n_train": int(train_idx.size),
            "test_sharpe": sharpe(tr, periods_per_year),
            "test_total_return": total_return(tr),
            "train_sharpe": sharpe(r[train_idx], periods_per_year),
        })
    return out


def summarize(folds):
    """Aggregate fold metrics into gauntlet criteria inputs."""
    test_sr = np.array([f["test_sharpe"] for f in folds])
    return {
        "n_folds": len(folds),
        "mean_test_sharpe": float(np.mean(test_sr)),
        "std_test_sharpe": float(np.std(test_sr, ddof=1)) if len(test_sr) > 1 else 0.0,
        "min_test_sharpe": float(np.min(test_sr)),
        "positive_fold_frac": float(np.mean(test_sr > 0)),
        "mean_test_total_return": float(np.mean([f["test_total_return"] for f in folds])),
        "folds": folds,
    }
