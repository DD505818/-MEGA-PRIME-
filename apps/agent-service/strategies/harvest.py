"""HARVEST — injected yield/carry rotation agent.

Requires yield_data with current_yield, benchmark_yield, and optional risk_score.
It does not connect to DeFi or staking protocols directly.
"""
from typing import Optional
import numpy as np

class HARVEST:
    def __init__(self, min_edge_bps: float = 150.0, max_risk_score: float = 0.45):
        self.name = "HARVEST"
        self.min_edge_bps = min_edge_bps
        self.max_risk_score = max_risk_score

    def generate_signal(self, daily_df=None, intraday_df=None, yield_data=None) -> Optional[dict]:
        if daily_df is None or len(daily_df) < 15 or not yield_data:
            return None
        current_yield = yield_data.get("current_yield")
        benchmark_yield = yield_data.get("benchmark_yield", 0.0)
        risk_score = float(yield_data.get("risk_score", 0.50))
        if current_yield is None:
            return None
        edge_bps = (float(current_yield) - float(benchmark_yield)) * 10000.0
        if edge_bps < self.min_edge_bps or risk_score > self.max_risk_score:
            return None
        price = float(daily_df["close"].iloc[-1])
        atr = self._atr(daily_df)
        if price <= 0 or not np.isfinite(atr) or atr <= 0:
            return None
        confidence = min(0.80, 0.50 + edge_bps / 1000.0 - risk_score / 2.0)
        return {"agent": self.name, "side": "BUY", "direction": "BUY", "entry": price, "stop": float(price - 2 * atr), "target": float(price + 2.5 * atr), "risk": float(2 * atr), "confidence": float(max(0.50, confidence)), "reason": f"Yield edge {edge_bps:.0f}bps with risk_score={risk_score:.2f}", "strategy_version": "14.2.1"}

    def risk_profile(self) -> dict:
        return {"max_position": 1, "risk_per_trade": 0.003, "preferred_regime": ["LOW_VOL", "SIDEWAYS"], "requires_external_data": "yield_data"}

    def explain(self) -> str:
        return "HARVEST emits carry/yield rotation candidates when injected yield edge clears risk-adjusted thresholds."

    @staticmethod
    def _atr(df, period: int = 14) -> float:
        high = df["high"]; low = df["low"]; close = df["close"]
        tr = np.maximum(np.maximum(high - low, np.abs(high - close.shift())), np.abs(low - close.shift()))
        return float(tr.rolling(period).mean().iloc[-1])
