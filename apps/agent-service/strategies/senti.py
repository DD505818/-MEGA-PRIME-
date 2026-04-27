"""SENTI — injected sentiment extreme agent for paper/backtest/live-safe integration.

This agent does not fetch external sentiment itself. Callers must inject a
sentiment score in sentiment_data={"score": 0..100}. Missing data returns None.
"""
from typing import Optional
import numpy as np

class SENTI:
    def __init__(self, fear_threshold: int = 25, greed_threshold: int = 75):
        self.name = "SENTI"
        self.fear_threshold = fear_threshold
        self.greed_threshold = greed_threshold

    def generate_signal(self, daily_df=None, intraday_df=None, sentiment_data=None) -> Optional[dict]:
        if daily_df is None or len(daily_df) < 15 or not sentiment_data:
            return None
        score = sentiment_data.get("score")
        if score is None or not 0 <= float(score) <= 100:
            return None
        price = float(daily_df["close"].iloc[-1])
        atr = self._atr(daily_df)
        if price <= 0 or not np.isfinite(atr) or atr <= 0:
            return None
        score = float(score)
        if score <= self.fear_threshold:
            confidence = min(0.90, 0.50 + (self.fear_threshold - score) / 50)
            return self._signal("BUY", price, price - 2.5 * atr, price + 3 * atr, 2.5 * atr, confidence, f"Extreme fear score={score:.0f}")
        if score >= self.greed_threshold:
            confidence = min(0.90, 0.50 + (score - self.greed_threshold) / 50)
            return self._signal("SELL", price, price + 2.5 * atr, price - 3 * atr, 2.5 * atr, confidence, f"Extreme greed score={score:.0f}")
        return None

    def risk_profile(self) -> dict:
        return {"max_position": 1, "risk_per_trade": 0.008, "preferred_regime": ["ALL"], "requires_external_data": "sentiment_data.score"}

    def explain(self) -> str:
        return "SENTI trades injected sentiment extremes contrarian: fear can trigger long candidates, greed can trigger short candidates."

    def _signal(self, side, entry, stop, target, risk, confidence, reason):
        return {"agent": self.name, "side": side, "direction": side, "entry": float(entry), "stop": float(stop), "target": float(target), "risk": float(risk), "confidence": float(confidence), "reason": reason, "strategy_version": "14.2.1"}

    @staticmethod
    def _atr(df, period: int = 14) -> float:
        high = df["high"]; low = df["low"]; close = df["close"]
        tr = np.maximum(np.maximum(high - low, np.abs(high - close.shift())), np.abs(low - close.shift()))
        return float(tr.rolling(period).mean().iloc[-1])
