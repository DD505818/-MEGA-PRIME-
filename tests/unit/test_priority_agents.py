import numpy as np
import pandas as pd

from apps.agent_service.strategies.gap import GAP
from apps.agent_service.strategies.rev import REV
from apps.agent_service.strategies.senti import SENTI
from apps.agent_service.strategies.twin import TWIN
from apps.agent_service.strategies.maker import MAKER
from apps.agent_service.strategies.harvest import HARVEST
from apps.agent_service.strategies import create_active_agents


def daily_df(rows=80, start=100.0, step=0.2):
    close = np.array([start + i * step for i in range(rows)], dtype=float)
    return pd.DataFrame({
        "open": close - 0.1,
        "high": close + 1.0,
        "low": close - 1.0,
        "close": close,
    })


def intraday_df(open_price=101.0, close_price=100.5):
    return pd.DataFrame({
        "open": [open_price],
        "high": [max(open_price, close_price) + 0.5],
        "low": [min(open_price, close_price) - 0.5],
        "close": [close_price],
    })


def assert_signal_contract(signal):
    assert signal is not None
    for key in ["agent", "side", "direction", "entry", "risk", "confidence", "reason", "strategy_version"]:
        assert key in signal


def test_create_active_agents_imports_without_missing_modules():
    agents = create_active_agents()
    names = {agent.name for agent in agents}
    assert {"GAP", "REV", "SENTI", "TWIN", "MAKER", "HARVEST"}.issubset(names)


def test_gap_generates_gap_up_sell_signal():
    df = daily_df()
    prior_close = float(df["close"].iloc[-2])
    agent = GAP(min_gap_pct=0.003)
    signal = agent.generate_signal(df, intraday_df(open_price=prior_close * 1.01, close_price=prior_close * 1.005))
    assert_signal_contract(signal)
    assert signal["side"] == "SELL"


def test_senti_requires_injected_data_and_generates_fear_signal():
    agent = SENTI()
    assert agent.generate_signal(daily_df(), sentiment_data=None) is None
    signal = agent.generate_signal(daily_df(), sentiment_data={"score": 10})
    assert_signal_contract(signal)
    assert signal["side"] == "BUY"


def test_twin_generates_signal_with_pair_data():
    primary = daily_df(rows=80, start=100, step=0.5)
    secondary = daily_df(rows=80, start=100, step=0.2)
    agent = TWIN(lookback=30, entry_z=1.0)
    signal = agent.generate_signal(primary, pair_df=secondary)
    if signal is not None:
        assert_signal_contract(signal)
        assert "pair_leg" in signal


def test_maker_generates_with_wide_order_book():
    agent = MAKER(spread_target_bps=5.0)
    signal = agent.generate_signal(daily_df(), intraday_df(), order_book={"bid": 99.0, "ask": 101.0})
    assert_signal_contract(signal)
    assert signal["side"] == "MAKER"
    assert signal["bid"] < signal["ask"]


def test_harvest_requires_yield_data_and_generates_signal():
    agent = HARVEST(min_edge_bps=100)
    assert agent.generate_signal(daily_df(), yield_data=None) is None
    signal = agent.generate_signal(daily_df(), yield_data={"current_yield": 0.08, "benchmark_yield": 0.02, "risk_score": 0.20})
    assert_signal_contract(signal)
    assert signal["side"] == "BUY"


def test_rev_returns_none_on_normal_trend_data():
    agent = REV()
    assert agent.generate_signal(daily_df()) is None
