"""ORACLE — on-chain data / crypto whale signal agent (scaffolded, interface-ready).

Tracks large on-chain transfers, exchange net flows, and whale wallet activity
to anticipate directional moves. Returns None until on-chain feed is integrated.
"""
from __future__ import annotations
from typing import Optional


class ORACLE:
    def __init__(self) -> None:
        self.name = "ORACLE"

    def generate_signal(self, daily_df=None, intraday_df=None, **kwargs) -> Optional[dict]:
        return None

    def risk_profile(self) -> dict:
        return {"max_position": 1, "risk_per_trade": 0.006, "preferred_regime": ["ALL"]}

    def explain(self) -> str:
        return "ORACLE reads on-chain whale movements and exchange net flows. Scaffolded pending blockchain data feed."
