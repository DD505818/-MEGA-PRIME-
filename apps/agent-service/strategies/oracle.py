"""ORACLE — on-chain data signal agent.

Aggregates blockchain analytics to anticipate directional moves:
- Exchange net flow (coins leaving exchanges = accumulation = bullish)
- Active address growth (network adoption momentum)
- Whale transaction volume (large holders moving capital)
- SOPR (Spent Output Profit Ratio): <1 = capitulation bottom signal

Requires onchain_data dict with the fields listed below.
"""
from __future__ import annotations
from typing import Optional
import numpy as np


class ORACLE:
    def __init__(
        self,
        min_addr_growth_pct: float = 0.20,
        exchange_outflow_threshold: float = -0.05,
        whale_volume_spike: float = 2.0,
        sopr_capitulation: float = 0.95,
        atr_period: int = 14,
    ) -> None:
        self.name = "ORACLE"
        self.min_addr_growth_pct = min_addr_growth_pct
        self.exchange_outflow_threshold = exchange_outflow_threshold
        self.whale_volume_spike = whale_volume_spike
        self.sopr_capitulation = sopr_capitulation
        self.atr_period = atr_period

    def generate_signal(
        self,
        daily_df=None,
        intraday_df=None,
        onchain_data: Optional[dict] = None,
        **kwargs,
    ) -> Optional[dict]:
        if daily_df is None or len(daily_df) < 14:
            return None
        if not onchain_data:
            return None

        price = float(daily_df["close"].iloc[-1])
        atr = self._calc_atr(daily_df)
        if not np.isfinite(atr) or atr <= 0 or price <= 0:
            return None

        addr_change = float(onchain_data.get("active_addr_change_7d", 0.0))
        exchange_reserve_change = float(onchain_data.get("exchange_reserve_change_7d", 0.0))
        whale_volume_ratio = float(onchain_data.get("whale_volume_ratio", 1.0))
        sopr = float(onchain_data.get("sopr", 1.0))
        nupl = float(onchain_data.get("nupl", 0.5))  # Net Unrealized Profit/Loss

        # Score each signal component
        bullish_score = 0.0
        bearish_score = 0.0
        reasons = []

        # Exchange outflow (coins leaving exchange = bullish accumulation)
        if exchange_reserve_change < self.exchange_outflow_threshold:
            bullish_score += 0.30
            reasons.append(f"exchange_outflow={exchange_reserve_change:.1%}")

        # Active address growth (network demand)
        if addr_change >= self.min_addr_growth_pct:
            bullish_score += 0.25
            reasons.append(f"addr_growth={addr_change:.0%}")

        # SOPR capitulation (below 1 = sellers selling at loss = near bottom)
        if sopr < self.sopr_capitulation:
            bullish_score += 0.25
            reasons.append(f"SOPR={sopr:.3f} (capitulation)")

        # Whale volume spike (large players accumulating)
        if whale_volume_ratio >= self.whale_volume_spike:
            bullish_score += 0.20
            reasons.append(f"whale_vol_ratio={whale_volume_ratio:.1f}x")

        # Bearish: NUPL euphoria (>0.75 = overextended)
        if nupl > 0.75:
            bearish_score += 0.40
            reasons.append(f"NUPL={nupl:.2f} (euphoria)")

        # Exchange inflow (coins entering exchange = selling pressure)
        if exchange_reserve_change > 0.05:
            bearish_score += 0.30
            reasons.append(f"exchange_inflow={exchange_reserve_change:.1%}")

        net_score = bullish_score - bearish_score
        if abs(net_score) < 0.25:
            return None  # not enough conviction

        side = "BUY" if net_score > 0 else "SELL"
        confidence = min(0.90, 0.55 + abs(net_score) * 0.7)
        stop_mult = 2.0 if side == "BUY" else -2.0
        target_mult = 2.5 if side == "BUY" else -2.5

        return {
            "agent": self.name,
            "side": side,
            "direction": side,
            "entry": float(price),
            "stop": float(price - stop_mult * atr) if side == "BUY" else float(price + abs(stop_mult) * atr),
            "target": float(price + target_mult * atr) if side == "BUY" else float(price - abs(target_mult) * atr),
            "risk": float(abs(stop_mult) * atr),
            "confidence": float(confidence),
            "reason": f"ORACLE: net_score={net_score:.2f}; " + "; ".join(reasons),
            "oracle_meta": {
                "active_addr_change_7d": addr_change,
                "exchange_reserve_change_7d": exchange_reserve_change,
                "whale_volume_ratio": whale_volume_ratio,
                "sopr": sopr,
                "nupl": nupl,
                "bullish_score": bullish_score,
                "bearish_score": bearish_score,
                "net_score": net_score,
            },
            "strategy_version": "17.0.0",
        }

    def risk_profile(self) -> dict:
        return {
            "max_position": 1,
            "risk_per_trade": 0.005,
            "preferred_regime": ["ALL"],
            "requires_external_data": "onchain_data",
        }

    def explain(self) -> str:
        return (
            "ORACLE reads on-chain blockchain analytics: exchange flows, "
            "active address growth, whale transactions, and SOPR/NUPL metrics. "
            "Composites these into a net bullish/bearish score above a conviction threshold."
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
