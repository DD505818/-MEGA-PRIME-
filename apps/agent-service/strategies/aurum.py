"""AURUM — precious metals basket rotation (XAU/XAG/XPT).

Ranks Gold, Silver, and Platinum by 20-day momentum, then trades
the strongest vs. weakest pair. Also detects standalone breakouts
when a single metal trends significantly.

Requires metals_data: dict mapping symbol → DataFrame with OHLC.
"""
from __future__ import annotations
from typing import Optional
import numpy as np


class AURUM:
    def __init__(
        self,
        momentum_period: int = 20,
        min_momentum_spread: float = 0.02,
        atr_stop_mult: float = 1.8,
        atr_period: int = 14,
    ) -> None:
        self.name = "AURUM"
        self.momentum_period = momentum_period
        self.min_momentum_spread = min_momentum_spread
        self.atr_stop_mult = atr_stop_mult
        self.atr_period = atr_period
        self._symbols = ["XAUUSD", "XAGUSD", "XPTUSD"]

    def generate_signal(
        self,
        daily_df=None,
        intraday_df=None,
        metals_data: Optional[dict] = None,
        **kwargs,
    ) -> Optional[dict]:
        # metals_data: {"XAUUSD": df, "XAGUSD": df, "XPTUSD": df}
        if not metals_data:
            # Fall back to primary df if only one metal provided
            if daily_df is None or len(daily_df) < self.momentum_period + self.atr_period:
                return None
            return self._single_metal_signal(daily_df, "XAUUSD")

        available = {
            sym: df for sym, df in metals_data.items()
            if df is not None and len(df) >= self.momentum_period + self.atr_period
        }
        if not available:
            return None

        # Compute 20-day price momentum for each metal
        momentum: dict[str, float] = {}
        for sym, df in available.items():
            close = df["close"].astype(float).to_numpy()
            m = (close[-1] - close[-self.momentum_period]) / close[-self.momentum_period]
            momentum[sym] = float(m)

        if len(momentum) < 2:
            sym = list(available.keys())[0]
            return self._single_metal_signal(available[sym], sym)

        ranked = sorted(momentum.items(), key=lambda x: x[1], reverse=True)
        long_sym, long_mom = ranked[0]
        short_sym, short_mom = ranked[-1]

        spread = long_mom - short_mom
        if spread < self.min_momentum_spread:
            return None

        long_df = available[long_sym]
        long_price = float(long_df["close"].iloc[-1])
        atr = self._calc_atr(long_df)
        if not np.isfinite(atr) or atr <= 0 or long_price <= 0:
            return None

        confidence = min(0.88, 0.55 + spread * 3.0)

        return {
            "agent": self.name,
            "side": "BUY",
            "direction": "METALS_ROTATION",
            "symbol": long_sym,
            "entry": float(long_price),
            "stop": float(long_price - self.atr_stop_mult * atr),
            "target": float(long_price + 2.2 * atr),
            "risk": float(self.atr_stop_mult * atr),
            "confidence": float(confidence),
            "reason": (
                f"AURUM: long {long_sym} ({long_mom:.1%}) "
                f"vs short {short_sym} ({short_mom:.1%}); "
                f"spread={spread:.1%}"
            ),
            "aurum_meta": {
                "long": long_sym,
                "short": short_sym,
                "long_momentum": long_mom,
                "short_momentum": short_mom,
                "momentum_spread": spread,
                "momentum_period": self.momentum_period,
            },
            "strategy_version": "17.0.0",
        }

    def _single_metal_signal(self, df, symbol: str) -> Optional[dict]:
        close = df["close"].astype(float).to_numpy()
        price = float(close[-1])
        mom = (close[-1] - close[-self.momentum_period]) / close[-self.momentum_period]
        atr = self._calc_atr(df)
        if not np.isfinite(atr) or atr <= 0 or abs(mom) < self.min_momentum_spread:
            return None
        side = "BUY" if mom > 0 else "SELL"
        stop = price - self.atr_stop_mult * atr if side == "BUY" else price + self.atr_stop_mult * atr
        target = price + 2.2 * atr if side == "BUY" else price - 2.2 * atr
        return {
            "agent": self.name,
            "side": side,
            "direction": side,
            "symbol": symbol,
            "entry": float(price),
            "stop": float(stop),
            "target": float(target),
            "risk": float(self.atr_stop_mult * atr),
            "confidence": min(0.82, 0.55 + abs(mom) * 3.0),
            "reason": f"AURUM: {symbol} {self.momentum_period}d momentum={mom:.1%}",
            "strategy_version": "17.0.0",
        }

    def risk_profile(self) -> dict:
        return {
            "max_position": 1,
            "risk_per_trade": 0.004,
            "preferred_regime": ["TRENDING", "MEAN_REVERTING", "HIGH_VOL"],
            "requires_external_data": "metals_data",
        }

    def explain(self) -> str:
        return (
            "AURUM ranks Gold, Silver, and Platinum by 20-day price momentum "
            "and rotates into the strongest while shorting the weakest. "
            "Minimum 2% momentum spread required to enter."
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
