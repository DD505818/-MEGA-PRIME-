"""GOLD — DXY/Gold inverse-correlation macro agent.

Requires macro_data with dxy_value and optionally vix_value.
Trades Gold (XAUUSD) using DXY divergence and inverse-correlation signals.
"""
from __future__ import annotations
from typing import Optional
import numpy as np


class GOLD:
    def __init__(
        self,
        dxy_drop_threshold: float = -0.5,
        dxy_rise_threshold: float = 0.5,
        correlation_window: int = 20,
        atr_stop_mult: float = 1.5,
    ) -> None:
        self.name = "GOLD"
        self.dxy_drop_threshold = dxy_drop_threshold
        self.dxy_rise_threshold = dxy_rise_threshold
        self.correlation_window = correlation_window
        self.atr_stop_mult = atr_stop_mult
        self._dxy_history: list[float] = []

    def generate_signal(
        self,
        daily_df=None,
        intraday_df=None,
        macro_data: Optional[dict] = None,
    ) -> Optional[dict]:
        if daily_df is None or len(daily_df) < max(self.correlation_window, 15):
            return None
        if not macro_data:
            return None

        dxy_value = macro_data.get("dxy_value")
        if dxy_value is None:
            return None

        dxy_value = float(dxy_value)
        self._dxy_history.append(dxy_value)
        if len(self._dxy_history) < 2:
            return None

        dxy_change_pct = ((dxy_value - self._dxy_history[-2]) / self._dxy_history[-2]) * 100.0

        gold_close = daily_df["close"].astype(float).to_numpy()
        price = float(gold_close[-1])
        atr = self._calc_atr(daily_df)
        if not np.isfinite(atr) or atr <= 0 or price <= 0:
            return None

        vix = float(macro_data.get("vix_value", 20.0))
        vix_boost = max(0.0, (vix - 20.0) / 40.0)

        if dxy_change_pct <= self.dxy_drop_threshold:
            confidence = min(0.88, 0.60 + abs(dxy_change_pct) / 5.0 + vix_boost)
            return self._signal(
                side="BUY",
                entry=price,
                stop=price - self.atr_stop_mult * atr,
                target=price + 2.0 * atr,
                risk=self.atr_stop_mult * atr,
                confidence=confidence,
                reason=f"DXY dropped {dxy_change_pct:.2f}% → Gold bullish; VIX={vix:.0f}",
            )

        if dxy_change_pct >= self.dxy_rise_threshold:
            confidence = min(0.85, 0.58 + abs(dxy_change_pct) / 5.0)
            return self._signal(
                side="SELL",
                entry=price,
                stop=price + self.atr_stop_mult * atr,
                target=price - 2.0 * atr,
                risk=self.atr_stop_mult * atr,
                confidence=confidence,
                reason=f"DXY rose {dxy_change_pct:.2f}% → Gold bearish; VIX={vix:.0f}",
            )

        return None

    def risk_profile(self) -> dict:
        return {
            "max_position": 1,
            "risk_per_trade": 0.006,
            "preferred_regime": ["CRISIS", "HIGH_VOL", "TRENDING"],
            "requires_external_data": "macro_data.dxy_value",
        }

    def explain(self) -> str:
        return (
            "GOLD exploits the historically inverse DXY-Gold relationship: "
            "DXY drops trigger Gold long entries; DXY spikes trigger short candidates. "
            "VIX amplifies confidence during risk-off events."
        )

    def _signal(
        self,
        side: str,
        entry: float,
        stop: float,
        target: float,
        risk: float,
        confidence: float,
        reason: str,
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
    def _calc_atr(df, period: int = 14) -> float:
        high = df["high"]
        low = df["low"]
        close = df["close"]
        tr = np.maximum(
            np.maximum(high - low, np.abs(high - close.shift())),
            np.abs(low - close.shift()),
        )
        return float(tr.rolling(period).mean().iloc[-1])
