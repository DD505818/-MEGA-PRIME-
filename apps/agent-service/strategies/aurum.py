"""AURUM — precious metals futures rotation agent (scaffolded, interface-ready).

Rotates between Gold, Silver, Platinum futures based on ratio mean reversion.
Returns None until futures data feed is integrated.
"""
from __future__ import annotations
from typing import Optional


class AURUM:
    def __init__(self) -> None:
        self.name = "AURUM"

    def generate_signal(self, daily_df=None, intraday_df=None, **kwargs) -> Optional[dict]:
        return None

    def risk_profile(self) -> dict:
        return {"max_position": 1, "risk_per_trade": 0.004, "preferred_regime": ["TRENDING", "MEAN_REVERTING"]}

    def explain(self) -> str:
        return "AURUM rotates between precious metals via ratio mean-reversion. Scaffolded pending futures feed."
