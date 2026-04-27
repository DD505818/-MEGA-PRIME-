"""
GAP — session gap-fill strategy.

Detects gaps between the prior daily close and the current session open, then
trades toward the prior close when early session price action confirms a fill.
The implementation is deterministic and returns None when required data is
missing or unsafe.
"""

from __future__ import annotations

from typing import Optional

import numpy as np


class GAP:
    def __init__(self, min_gap_pct: float = 0.003, atr_period: int = 14) -> None:
        self.name = "GAP"
        self.min_gap_pct = min_gap_pct
        self.atr_period = atr_period
        self.filled = False

    def generate_signal(self, daily_df, intraday_df) -> Optional[dict]:
        if daily_df is None or intraday_df is None:
            return None
        if len(daily_df) < max(2, self.atr_period + 1) or len(intraday_df) < 1:
            return None

        prior_close = float(daily_df["close"].iloc[-2])
        if prior_close <= 0:
            return None

        current_open = float(intraday_df["open"].iloc[0])
        current_bar = intraday_df.iloc[-1]
        current_close = float(current_bar["close"])
        gap_pct = (current_open - prior_close) / prior_close

        if abs(gap_pct) < self.min_gap_pct or self.filled:
            return None

        atr = self._calc_atr(daily_df, self.atr_period)
        if not np.isfinite(atr) or atr <= 0:
            return None

        if gap_pct < -self.min_gap_pct and current_close > current_open:
            self.filled = True
            return self._signal(
                side="BUY",
                entry=current_close,
                stop=current_open - 1.5 * atr,
                target=prior_close,
                risk=1.5 * atr,
                confidence=min(0.90, max(0.50, abs(gap_pct) * 10.0)),
                reason=f"Gap fill: {gap_pct * 100:.2f}% gap down, buying toward prior close",
            )

        if gap_pct > self.min_gap_pct and current_close < current_open:
            self.filled = True
            return self._signal(
                side="SELL",
                entry=current_close,
                stop=current_open + 1.5 * atr,
                target=prior_close,
                risk=1.5 * atr,
                confidence=min(0.90, max(0.50, gap_pct * 10.0)),
                reason=f"Gap fill: {gap_pct * 100:.2f}% gap up, selling toward prior close",
            )

        return None

    def risk_profile(self) -> dict:
        return {
            "max_position": 1,
            "risk_per_trade": 0.005,
            "stop_multiplier": 1.5,
            "preferred_regime": ["ALL"],
            "min_gap_pct": self.min_gap_pct,
        }

    def explain(self) -> str:
        return (
            "GAP trades statistically meaningful session gaps back toward the "
            "prior close after intraday confirmation. It is designed as a "
            "controlled mean-reversion strategy with one active fill attempt per session."
        )

    def _signal(self, side: str, entry: float, stop: float, target: float, risk: float, confidence: float, reason: str) -> dict:
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
            "strategy_version": "14.2.1",
        }

    @staticmethod
    def _calc_atr(df, period: int = 14) -> float:
        high = df["high"]
        low = df["low"]
        close = df["close"]
        tr1 = high - low
        tr2 = np.abs(high - close.shift())
        tr3 = np.abs(low - close.shift())
        tr = np.maximum(np.maximum(tr1, tr2), tr3)
        return float(tr.rolling(period).mean().iloc[-1])
