"""Shared fixtures: synthetic validation-lab reports (never the lab itself)."""

import copy

import pytest

from midas.promote import STAGE_ORDER
from midas.registry import PromotionRegistry
from midas.thresholds import Thresholds

# Real frozen hash from ~/workspace/datasets/omega-prime/coinbase/MANIFEST.json
COINBASE_1H_SHA256 = (
    "4aa57aa2c87a24c2033f57db715391c5b95be7d39a4859f457abed31d54f039d"
)
DATASET_DIR = "/home/hatch/workspace/datasets/omega-prime"


def make_pass_report(**overrides):
    stages = {}
    for name in STAGE_ORDER:
        stages[name] = {"status": "PASS", "metrics": {}, "threshold": None}
    stages["costs"]["metrics"] = {"net_sharpe": 1.8, "net_ann_return": 0.42}
    stages["walkforward"]["metrics"] = {
        "mean_test_sharpe": 0.9,
        "positive_fold_frac": 0.8,
    }
    stages["nulls"]["metrics"] = {
        "permutation": {"p_value": 0.01},
        "random_timing": {"p_value": 0.02},
    }
    stages["bootstrap"]["metrics"] = {"sharpe_ci": [0.4, 2.1]}
    stages["montecarlo"]["metrics"] = {
        "regime_switching": {"prob_total_return_negative": 0.2}
    }
    stages["overfit"]["metrics"] = {"deflated_sharpe": 1.2}
    report = {
        "candidate": "test-strategy",
        "verdict": "PASS",
        "failed_stage": None,
        "config": {
            "min_net_sharpe": 0.0,
            "min_oos_sharpe": 0.5,
            "min_positive_fold_frac": 0.6,
            "max_pvalue": 0.05,
            "min_dsr": 0.95,
            "sensitivity_min_frac": 0.5,
            "max_mc_loss_prob": 0.5,
        },
        "data": {
            "file": "coinbase_BTC-USD_1h.csv",
            "sha256": COINBASE_1H_SHA256,
            "rows": 26270,
        },
        "stages": stages,
        "doctrine": "test",
    }
    for k, v in overrides.items():
        report[k] = copy.deepcopy(v)
    return report


@pytest.fixture
def registry(tmp_path):
    reg = PromotionRegistry(str(tmp_path / "midas.db"))
    yield reg
    reg.close()


@pytest.fixture
def thresholds():
    return Thresholds()  # documented defaults, no env dependence
