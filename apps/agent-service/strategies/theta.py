"""THETA — options theta-decay harvesting agent.

Systematically identifies and sells time-value-rich options when:
- IV Rank > 70 (top 30% of historical IV range)
- Days to expiry (DTE) ≤ 45 (sweet spot for theta decay)
- Premium exceeds minimum edge threshold after bid/ask spread

Requires options_chain data with per-strike IV and greeks.
Does NOT operate in high-vol (IV rank > 90) — risk too asymmetric.
"""
from __future__ import annotations
from typing import Optional
import numpy as np


class THETA:
    def __init__(
        self,
        min_iv_rank: float = 70.0,
        max_iv_rank: float = 90.0,
        max_dte: int = 45,
        min_theta_per_dollar: float = 0.001,
        min_premium_pct: float = 0.005,
        atr_period: int = 14,
    ) -> None:
        self.name = "THETA"
        self.min_iv_rank = min_iv_rank
        self.max_iv_rank = max_iv_rank
        self.max_dte = max_dte
        self.min_theta_per_dollar = min_theta_per_dollar
        self.min_premium_pct = min_premium_pct
        self.atr_period = atr_period

    def generate_signal(
        self,
        daily_df=None,
        intraday_df=None,
        options_chain: Optional[list] = None,
        **kwargs,
    ) -> Optional[dict]:
        if daily_df is None or len(daily_df) < 30:
            return None
        if not options_chain:
            return None

        price = float(daily_df["close"].iloc[-1])
        atr = self._calc_atr(daily_df)
        if not np.isfinite(atr) or atr <= 0 or price <= 0:
            return None

        best_strike = self._find_best_strike(options_chain, price)
        if best_strike is None:
            return None

        iv_rank = float(best_strike.get("iv_rank", 0))
        dte = int(best_strike.get("dte", 999))
        premium = float(best_strike.get("mid_price", 0))
        theta = float(best_strike.get("theta", 0))
        strike = float(best_strike.get("strike", price))
        option_type = best_strike.get("type", "put")

        if iv_rank < self.min_iv_rank or iv_rank > self.max_iv_rank:
            return None
        if dte > self.max_dte:
            return None
        if premium <= 0 or premium / price < self.min_premium_pct:
            return None
        if theta == 0 or abs(theta) / premium < self.min_theta_per_dollar:
            return None

        # Confidence scaled by IV rank percentile and DTE efficiency
        dte_factor = max(0.5, 1.0 - dte / 90.0)
        iv_factor = (iv_rank - self.min_iv_rank) / (self.max_iv_rank - self.min_iv_rank)
        confidence = min(0.87, 0.60 + iv_factor * 0.2 + dte_factor * 0.1)

        # Max loss on short put: strike - premium (bounded by ATR-based risk)
        max_loss = min(strike * 0.10, 3.0 * atr)

        return {
            "agent": self.name,
            "side": "SELL",
            "direction": "SHORT_THETA",
            "entry": float(premium),
            "stop": float(premium * 3.0),
            "target": 0.0,
            "risk": float(max_loss),
            "confidence": float(confidence),
            "reason": (
                f"THETA: sell {option_type} strike={strike:.2f} DTE={dte} "
                f"IV_rank={iv_rank:.0f} premium={premium:.4f} theta={theta:.4f}"
            ),
            "theta_meta": {
                "strike": strike,
                "option_type": option_type,
                "dte": dte,
                "iv_rank": iv_rank,
                "premium": premium,
                "theta": theta,
                "underlying_price": price,
            },
            "strategy_version": "17.0.0",
        }

    def _find_best_strike(
        self, chain: list, price: float
    ) -> Optional[dict]:
        candidates = [
            s for s in chain
            if isinstance(s, dict)
            and s.get("dte", 999) <= self.max_dte
            and self.min_iv_rank <= float(s.get("iv_rank", 0)) <= self.max_iv_rank
            and float(s.get("mid_price", 0)) > 0
            and float(s.get("theta", 0)) < 0
        ]
        if not candidates:
            return None
        # Select strike with highest theta/premium ratio (decay efficiency)
        return max(
            candidates,
            key=lambda s: abs(float(s.get("theta", 0))) / max(float(s.get("mid_price", 0.0001)), 0.0001),
        )

    def risk_profile(self) -> dict:
        return {
            "max_position": 2,
            "risk_per_trade": 0.008,
            "preferred_regime": ["SIDEWAYS", "MEAN_REVERTING"],
            "requires_external_data": "options_chain",
            "paper_only_until_live_options_feed": True,
        }

    def explain(self) -> str:
        return (
            "THETA harvests time-value decay by selling options with IV Rank 70–90, "
            "DTE ≤ 45 days, and high theta/premium decay efficiency. "
            "Avoids extreme IV (>90 rank) where tail risk dominates."
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
