import numpy as np

from validation_lab import walkforward


def test_no_overlap_with_embargo():
    n, folds = 1000, 5
    splits = walkforward.purged_cv_splits(n, folds, embargo_pct=0.01,
                                          label_horizon=1)
    assert len(splits) == folds
    for train, test in splits:
        # purge zone: [t0-1+1, t1) and embargo [t1, t1+emb) excluded from train
        assert not np.intersect1d(train, test).size
        t0, t1 = test[0], test[-1] + 1
        emb = max(1, int(n * 0.01))
        assert not np.any((train >= t1) & (train < t1 + emb))
        # every test index covered exactly once across folds
    all_test = np.concatenate([t for _, t in splits])
    assert all_test.size == n


def test_purge_removes_label_overlap():
    # label_horizon=5: train samples whose label touches the test fold are gone
    n = 200
    splits = walkforward.purged_cv_splits(n, 4, embargo_pct=0.0,
                                          label_horizon=5)
    train, test = splits[1]
    t0 = test[0]
    assert not np.any((train >= t0 - 4) & (train < t0))


def test_fold_metrics_shape():
    rng = np.random.default_rng(0)
    r = rng.normal(0.0001, 0.01, size=500)
    splits = walkforward.purged_cv_splits(500, 4)
    folds = walkforward.fold_metrics(r, splits, 24 * 365)
    s = walkforward.summarize(folds)
    assert s["n_folds"] == 4
    assert 0.0 <= s["positive_fold_frac"] <= 1.0
