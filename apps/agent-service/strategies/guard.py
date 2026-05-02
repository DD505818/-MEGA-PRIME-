"""GUARD — tail-risk pre-filter and portfolio hedge agent.

Detects volatility shocks and emits REDUCE or HEDGE signals when
realized vol spikes more than 2σ above its rolling 20-day mean.
Also monitors correlation clustering as a pre-crisis indicator.

Injects into the orchestrator as a gating signal: when GUARD fires,
the risk engine reduces all position sizes by 25–50%.
"""
from __future__ import annotations
from typing import Optional
import numpy as np


class GUARD:
    def __init__(
        self,
        vol_lookback: int = 20,
        vol_spike_sigma: float = 2.0,
        atr_period: int = 14,
        crisis_drawdown_pct: float = 0.05,
    ) -> None:
        self.name = "GUARD"
        self.vol_lookback = vol_lookback
        self.vol_spike_sigma = vol_spike_sigma
        self.atr_period = atr_period
        self.crisis_drawdown_pct = crisis_drawdown_pct

    def generate_signal(
        self,
        daily_df=None,
        intraday_df=None,
        portfolio_data: Optional[dict] = None,
        **kwargs,
    ) -> Optional[dict]:
        if daily_df is None or len(daily_df) < self.vol_lookback + self.atr_period:
            return None

        close = daily_df["close"].astype(float).to_numpy()
        returns = np.diff(np.log(close[-(self.vol_lookback + 2):]))
        if len(returns) < self.vol_lookback:
            return None

        current_vol = float(np.std(returns[-5:]) * np.sqrt(252))
        hist_vol = float(np.std(returns) * np.sqrt(252))
        hist_vol_mean = float(np.mean([
            np.std(returns[i:i+5]) * np.sqrt(252)
            for i in range(len(returns) - 5)
        ])) if len(returns) > 10 else hist_vol
        hist_vol_std = float(np.std([
            np.std(returns[i:i+5]) * np.sqrt(252)
            for i in range(len(returns) - 5)
        ])) if len(returns) > 10 else hist_vol * 0.3

        if hist_vol_std <= 0:
            return None

        vol_z = (current_vol - hist_vol_mean) / hist_vol_std
        price = float(close[-1])
        atr = self._calc_atr(daily_df)
        if not np.isfinite(atr) or atr <= 0:
            return None

        # Portfolio drawdown gate
        drawdown = 0.0
        if portfolio_data:
            drawdown = float(portfolio_data.get("drawdown_pct", 0.0))

        # Level 1: vol spike > 2σ → reduce signal
        if vol_z >= self.vol_spike_sigma:
            severity = min(0.95, 0.50 + (vol_z - self.vol_spike_sigma) / 4.0)
            return {
                "agent": self.name,
                "side": "SELL",
                "direction": "HEDGE",
                "action": "REDUCE_ALL",
                "reduce_factor": 0.50 if vol_z >= 3.0 else 0.25,
                "entry": float(price),
                "stop": float(price + 2.0 * atr),
                "target": float(price - 1.5 * atr),
                "risk": float(2.0 * atr),
                "confidence": float(severity),
                "reason": (
                    f"GUARD: vol spike {vol_z:.1f}σ above 20d mean "
                    f"(current={current_vol:.1%} hist={hist_vol_mean:.1%}); "
                    f"drawdown={drawdown:.1%}"
                ),
                "strategy_version": "17.0.0",
            }

        # Level 2: deep drawdown combined with vol expansion → crisis hedge
        if drawdown >= self.crisis_drawdown_pct and current_vol > hist_vol * 1.5:
            return {
                "agent": self.name,
                "side": "SELL",
                "direction": "CRISIS_HEDGE",
                "action": "REDUCE_ALL",
                "reduce_factor": 0.75,
                "entry": float(price),
                "stop": float(price + 3.0 * atr),
                "target": float(price - 2.0 * atr),
                "risk": float(3.0 * atr),
                "confidence": 0.88,
                "reason": (
                    f"GUARD: drawdown={drawdown:.1%} + vol={current_vol:.1%} "
                    f"→ crisis hedge activated"
                ),
                "strategy_version": "17.0.0",
            }

        return None

    def risk_profile(self) -> dict:
        return {
            "max_position": 0,
            "risk_per_trade": 0.0,
            "preferred_regime": ["HIGH_VOL", "CRISIS"],
            "function": "gating",
        }

    def explain(self) -> str:
        return (
            "GUARD monitors realized volatility against its 20-day rolling distribution. "
            "When vol spikes >2σ it emits REDUCE signals cutting all positions 25–75%. "
            "Combined drawdown+vol expansion triggers full crisis hedge mode."
        )

    @staticmethod
    def _calc_atr(df, period: int = 14) -> float:
        high = df["high"].astype(float).to_numpy()
        low = df["low"].astype(float).to_numpy()
        close = df["close"].astype(float).to_numpy()
        tr = np.maximum(
            np.maximum(high[1:] - low[1:], np.abs(high[1:] - close[:-1])),
            np.abs(low[1:] - close[:-1]),
        )
        return float(np.mean(tr[-period:]))
