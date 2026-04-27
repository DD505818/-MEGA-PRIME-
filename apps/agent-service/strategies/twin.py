"""TWIN — pairs/spread strategy using injected secondary asset data."""
from typing import Optional
import numpy as np

class TWIN:
    def __init__(self, lookback: int = 50, entry_z: float = 2.0, exit_z: float = 0.5):
        self.name = "TWIN"
        self.lookback = lookback
        self.entry_z = entry_z
        self.exit_z = exit_z
        self.position = 0

    def generate_signal(self, daily_df=None, intraday_df=None, pair_df=None) -> Optional[dict]:
        if daily_df is None or pair_df is None:
            return None
        if len(daily_df) < self.lookback or len(pair_df) < self.lookback:
            return None
        primary = daily_df["close"].astype(float).to_numpy()
        secondary = pair_df["close"].astype(float).to_numpy()
        n = min(len(primary), len(secondary))
        primary = primary[-n:]
        secondary = secondary[-n:]
        if np.any(primary <= 0) or np.any(secondary <= 0):
            return None
        spread = np.log(primary) - np.log(secondary)
        window = spread[-self.lookback:]
        mean = float(np.mean(window))
        std = float(np.std(window))
        if std <= 1e-8 or not np.isfinite(std):
            return None
        z = float((spread[-1] - mean) / std)
        price = float(primary[-1])
        atr = self._atr_from_prices(primary)
        if atr <= 0 or not np.isfinite(atr):
            return None
        if self.position == 0 and z > self.entry_z:
            self.position = -1
            return self._signal("SELL", price, price + 1.5 * atr, price - atr, 1.5 * atr, min(0.90, abs(z) / 3.5), f"Spread rich z={z:.2f}; short primary, long secondary", "BUY_SECONDARY")
        if self.position == 0 and z < -self.entry_z:
            self.position = 1
            return self._signal("BUY", price, price - 1.5 * atr, price + atr, 1.5 * atr, min(0.90, abs(z) / 3.5), f"Spread cheap z={z:.2f}; long primary, short secondary", "SELL_SECONDARY")
        if self.position == 1 and z > -self.exit_z:
            self.position = 0
            return self._signal("SELL", price, price, price, 0.0, 1.0, f"Spread converged z={z:.2f}; close long spread", "CLOSE_SECONDARY")
        if self.position == -1 and z < self.exit_z:
            self.position = 0
            return self._signal("BUY", price, price, price, 0.0, 1.0, f"Spread converged z={z:.2f}; close short spread", "CLOSE_SECONDARY")
        return None

    def risk_profile(self) -> dict:
        return {"max_position": 1, "risk_per_trade": 0.004, "preferred_regime": ["ALL"], "requires_external_data": "pair_df"}

    def explain(self) -> str:
        return "TWIN trades log-spread extremes between a primary and secondary asset, with explicit secondary-leg metadata."

    def _signal(self, side, entry, stop, target, risk, confidence, reason, pair_leg):
        return {"agent": self.name, "side": side, "direction": side, "entry": float(entry), "stop": float(stop), "target": float(target), "risk": float(risk), "confidence": float(confidence), "reason": reason, "pair_leg": pair_leg, "strategy_version": "14.2.1"}

    @staticmethod
    def _atr_from_prices(prices, period: int = 14) -> float:
        if len(prices) < 2:
            return 0.0
        return float(np.mean(np.abs(np.diff(prices))[-period:]))
