"""SURGE — momentum breakout strategy with ATR-based pyramiding.

Detects high-momentum breakouts above/below a rolling channel (Donchian-style)
confirmed by an ATR expansion filter. Uses the GEPA parameter search space
breakout_atr_mult in [1.5, 2.0, 2.5, 3.0] (default 2.0).
"""
from __future__ import annotations
from typing import Optional
import numpy as np


class Surge:
    def __init__(
        self,
        channel_period: int = 20,
        breakout_atr_mult: float = 2.0,
        atr_expansion_min: float = 1.2,
        pyramid_trigger_r: float = 1.0,
        atr_period: int = 14,
    ) -> None:
        self.name = "Surge"
        self.channel_period = channel_period
        self.breakout_atr_mult = breakout_atr_mult
        self.atr_expansion_min = atr_expansion_min
        self.pyramid_trigger_r = pyramid_trigger_r
        self.atr_period = atr_period
        self._position: Optional[dict] = None

    def generate_signal(self, daily_df=None, intraday_df=None) -> Optional[dict]:
        df = intraday_df if intraday_df is not None and len(intraday_df) >= self.channel_period + self.atr_period else daily_df
        if df is None or len(df) < self.channel_period + self.atr_period:
            return None

        close = df["close"].astype(float).to_numpy()
        high = df["high"].astype(float).to_numpy()
        low = df["low"].astype(float).to_numpy()

        price = close[-1]
        if price <= 0:
            return None

        # ATR with chained np.maximum (no lookahead)
        atr = self._calc_atr_chain(high, low, close, self.atr_period)
        if not np.isfinite(atr) or atr <= 0:
            return None

        atr_mean = np.mean(
            [self._calc_atr_chain(high[:i+1], low[:i+1], close[:i+1], self.atr_period)
             for i in range(len(close) - 20, len(close) - 1)
             if i > self.atr_period]
        ) if len(close) > self.atr_period + 20 else atr

        atr_expanding = atr >= self.atr_expansion_min * atr_mean if atr_mean > 0 else False

        # Donchian channel (look only at bars[:-1] to avoid lookahead)
        lookback = close[-(self.channel_period + 1):-1]
        high_lookback = high[-(self.channel_period + 1):-1]
        low_lookback = low[-(self.channel_period + 1):-1]

        upper = float(np.max(high_lookback))
        lower = float(np.min(low_lookback))

        # Handle active position pyramiding
        if self._position is not None:
            return self._check_pyramid(price, atr)

        # Breakout entry
        if price > upper and atr_expanding:
            entry = price
            stop = price - self.breakout_atr_mult * atr
            target = price + 2.5 * atr
            self._position = {
                "side": "BUY", "entry": entry, "stop": stop,
                "atr": atr, "pyramids": 0,
            }
            return self._signal(
                "BUY", entry, stop, target, self.breakout_atr_mult * atr,
                min(0.85, 0.60 + (price - upper) / atr * 0.05),
                f"Donchian breakout above {upper:.2f}; ATR={atr:.4f} expanding",
            )

        if price < lower and atr_expanding:
            entry = price
            stop = price + self.breakout_atr_mult * atr
            target = price - 2.5 * atr
            self._position = {
                "side": "SELL", "entry": entry, "stop": stop,
                "atr": atr, "pyramids": 0,
            }
            return self._signal(
                "SELL", entry, stop, target, self.breakout_atr_mult * atr,
                min(0.85, 0.60 + (lower - price) / atr * 0.05),
                f"Donchian breakdown below {lower:.2f}; ATR={atr:.4f} expanding",
            )

        return None

    def _check_pyramid(self, price: float, atr: float) -> Optional[dict]:
        pos = self._position
        if pos["pyramids"] >= 3:
            return None

        entry = pos["entry"]
        r = atr * pos.get("atr", atr)
        gain = (price - entry) if pos["side"] == "BUY" else (entry - price)

        if gain >= self.pyramid_trigger_r * atr * (pos["pyramids"] + 1):
            pos["pyramids"] += 1
            pos["stop"] = entry  # trail stop to breakeven
            return self._signal(
                pos["side"],
                price,
                pos["stop"],
                price + 2.0 * atr if pos["side"] == "BUY" else price - 2.0 * atr,
                atr,
                0.78,
                f"Surge pyramid #{pos['pyramids']}; gain={gain:.4f}",
            )
        return None

    def close_position(self) -> None:
        self._position = None

    def risk_profile(self) -> dict:
        return {
            "max_position": 1,
            "risk_per_trade": 0.005,
            "preferred_regime": ["TRENDING", "HIGH_VOL"],
            "breakout_atr_mult": self.breakout_atr_mult,
            "max_pyramids": 3,
        }

    def explain(self) -> str:
        return (
            "Surge detects momentum breakouts beyond the 20-bar Donchian channel "
            "with ATR expansion confirmation. Supports up to 3 pyramiding entries "
            "as the trend extends, trailing stop to breakeven at each add."
        )

    def _signal(
        self, side, entry, stop, target, risk, confidence, reason
    ) -> dict:
        return {
            "agent": self.name,
            "side": side,
            "direction": side,
            "entry": float(entry),
            "stop": float(stop),
            "target": float(target),
            "risk": float(risk),
            "confidence": float(confidence),
            "reason": reason,
            "strategy_version": "17.0.0",
        }

    @staticmethod
    def _calc_atr_chain(high, low, close, period: int) -> float:
        if len(close) < period + 1:
            return float(np.mean(high[-period:] - low[-period:]))
        tr = np.maximum(
            np.maximum(high[1:] - low[1:], np.abs(high[1:] - close[:-1])),
            np.abs(low[1:] - close[:-1]),
        )
        return float(np.mean(tr[-period:]))
