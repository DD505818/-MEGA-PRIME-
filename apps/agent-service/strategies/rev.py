"""REV — deterministic RSI/Bollinger mean-reversion agent for paper/backtest use."""
from typing import Optional
import numpy as np

class REV:
    def __init__(self, rsi_period: int = 14, bb_period: int = 20):
        self.name = "REV"
        self.rsi_period = rsi_period
        self.bb_period = bb_period

    def generate_signal(self, daily_df, intraday_df=None) -> Optional[dict]:
        if daily_df is None or len(daily_df) < max(self.rsi_period, self.bb_period, 14) + 1:
            return None
        close = daily_df["close"].astype(float).to_numpy()
        price = float(close[-1])
        atr = self._atr(daily_df)
        if price <= 0 or not np.isfinite(atr) or atr <= 0:
            return None
        rsi = self._rsi(close)
        mean = float(np.mean(close[-self.bb_period:]))
        std = float(np.std(close[-self.bb_period:]))
        if not np.isfinite(rsi) or std <= 0:
            return None
        lower = mean - 2 * std
        upper = mean + 2 * std
        if rsi < 28 and price <= lower:
            return self._signal("BUY", price, price - 2 * atr, mean, 2 * atr, 0.70, f"RSI={rsi:.1f} below lower band")
        if rsi > 72 and price >= upper:
            return self._signal("SELL", price, price + 2 * atr, mean, 2 * atr, 0.70, f"RSI={rsi:.1f} above upper band")
        return None

    def risk_profile(self) -> dict:
        return {"max_position": 1, "risk_per_trade": 0.005, "preferred_regime": ["MEAN_REVERSION", "SIDEWAYS"]}

    def explain(self) -> str:
        return "REV uses RSI extremes plus Bollinger Bands to identify controlled mean-reversion candidates."

    def _signal(self, side, entry, stop, target, risk, confidence, reason):
        return {"agent": self.name, "side": side, "direction": side, "entry": float(entry), "stop": float(stop), "target": float(target), "risk": float(risk), "confidence": float(confidence), "reason": reason, "strategy_version": "14.2.1"}

    def _rsi(self, close):
        diff = np.diff(close[-self.rsi_period-1:])
        gain = np.where(diff > 0, diff, 0.0).mean()
        loss = np.where(diff < 0, -diff, 0.0).mean()
        if loss == 0:
            return 100.0
        return float(100 - (100 / (1 + gain / loss)))

    @staticmethod
    def _atr(df, period: int = 14) -> float:
        high = df["high"]; low = df["low"]; close = df["close"]
        tr = np.maximum(np.maximum(high - low, np.abs(high - close.shift())), np.abs(low - close.shift()))
        return float(tr.rolling(period).mean().iloc[-1])
