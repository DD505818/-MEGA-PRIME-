"""GUARD — tail-risk hedge agent (scaffolded, interface-ready).

Intended to buy protective puts / inverse ETFs when portfolio tail risk exceeds
CVaR thresholds. Returns None until ML tail-risk model is integrated.
"""
from __future__ import annotations
from typing import Optional


class GUARD:
    def __init__(self) -> None:
        self.name = "GUARD"

    def generate_signal(self, daily_df=None, intraday_df=None, **kwargs) -> Optional[dict]:
        return None

    def risk_profile(self) -> dict:
        return {"max_position": 1, "risk_per_trade": 0.005, "preferred_regime": ["CRISIS", "HIGH_VOL"]}

    def explain(self) -> str:
        return "GUARD buys tail-risk hedges when CVaR breaches portfolio thresholds. Scaffolded pending ML integration."
