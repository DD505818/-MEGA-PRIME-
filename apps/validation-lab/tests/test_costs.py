import numpy as np
import pandas as pd

from validation_lab import costs


def test_no_trades_no_cost():
    gross = np.full(100, 0.001)
    pos = np.zeros(100)
    r = costs.apply_costs(gross, pos, fee_bps=10, spread_bps=5)
    assert r["total_cost"] == 0.0
    assert abs(r["net_total"] - r["gross_total"]) < 1e-12


def test_turnover_cost_math():
    # one round trip: 0 -> 1 -> 0 ; cost = 2 * (10+5)bps = 30bps
    gross = np.zeros(10)
    pos = np.array([0, 1, 1, 1, 0, 0, 0, 0, 0, 0], dtype=float)
    r = costs.apply_costs(gross, pos, fee_bps=10, spread_bps=5)
    assert abs(r["total_trade_cost"] - 0.0030) < 1e-12
    assert r["n_trades"] == 2


def test_funding_accrues():
    gross = np.zeros(24)
    pos = np.ones(24)
    r = costs.apply_costs(gross, pos, fee_bps=0, spread_bps=0,
                          funding_annual_bps=365.0, periods_per_year=24 * 365)
    # 365bps/yr = 3.65%/yr; per hour 3.65%/8760; *24h = 1e-4
    assert abs(r["total_funding_cost"] - 0.0001) < 1e-12


def test_stress_monotone():
    rng = np.random.default_rng(0)
    gross = rng.normal(0.0002, 0.005, size=1000)
    pos = rng.choice([-1.0, 0.0, 1.0], size=1000)
    s = costs.cost_stress(gross, pos, fee_bps=10, spread_bps=5)
    assert s["1x"]["net_sharpe"] >= s["2x"]["net_sharpe"] >= s["3x"]["net_sharpe"]
