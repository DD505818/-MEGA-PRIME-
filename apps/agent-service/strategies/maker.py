"""MAKER — spread-capture candidate generator.

Requires either order_book={bid, ask} or conservative OHLC-derived spread proxy.
This agent emits a maker quote candidate and must still pass AEGIS/execution checks.
"""
from typing import Optional
import numpy as np

class MAKER:
    def __init__(self, spread_target_bps: float = 8.0, max_position: float = 0.5):
        self.name = "MAKER"
        self.spread_target_bps = spread_target_bps
        self.max_position = max_position
        self.position = 0.0

    def generate_signal(self, daily_df=None, intraday_df=None, order_book=None) -> Optional[dict]:
        if intraday_df is None or len(intraday_df) < 1:
            return None
        bar = intraday_df.iloc[-1]
        if order_book and order_book.get("bid") and order_book.get("ask"):
            bid = float(order_book["bid"]); ask = float(order_book["ask"])
        else:
            bid = float(bar["low"]); ask = float(bar["high"])
        if bid <= 0 or ask <= bid:
            return None
        mid = (bid + ask) / 2.0
        spread_bps = ((ask - bid) / mid) * 10000.0
        if spread_bps < self.spread_target_bps or abs(self.position) >= self.max_position:
            return None
        atr = self._atr(daily_df) if daily_df is not None and len(daily_df) >= 15 else mid * 0.005
        if not np.isfinite(atr) or atr <= 0:
            return None
        quote_size = min(0.01, max(0.0, self.max_position - abs(self.position)))
        return {"agent": self.name, "side": "MAKER", "direction": "MAKER", "entry": float(mid), "bid": float(max(0.0, mid - 0.1 * atr)), "ask": float(mid + 0.1 * atr), "size": float(quote_size), "risk": float(0.2 * atr), "confidence": float(min(0.85, spread_bps / 20.0)), "reason": f"Spread capture candidate spread={spread_bps:.1f}bps", "strategy_version": "14.2.1"}

    def risk_profile(self) -> dict:
        return {"max_position": self.max_position, "risk_per_trade": 0.002, "preferred_regime": ["SIDEWAYS", "LOW_VOL"], "requires_execution_support": "dual-sided maker quoting"}

    def explain(self) -> str:
        return "MAKER emits bounded spread-capture quote candidates; execution must enforce inventory, cancel/replace, and venue constraints."

    @staticmethod
    def _atr(df, period: int = 14) -> float:
        high = df["high"]; low = df["low"]; close = df["close"]
        tr = np.maximum(np.maximum(high - low, np.abs(high - close.shift())), np.abs(low - close.shift()))
        return float(tr.rolling(period).mean().iloc[-1])
