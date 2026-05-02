"""STAKE — staking yield rotation optimizer.

Ranks staking opportunities across chains by risk-adjusted APY.
Uses a Sharpe-proxy (APY / protocol_risk_score) to select the
optimal allocation. Emits entry/rotation signals when a materially
better yield becomes available.

Requires staking_yields: list of dicts with chain, apy, risk_score, tvl_usd.
"""
from __future__ import annotations
from typing import Optional
import numpy as np


class STAKE:
    def __init__(
        self,
        min_apy: float = 0.04,
        min_tvl_usd: float = 100_000_000.0,
        max_risk_score: float = 0.50,
        rotation_threshold_pct: float = 0.015,
        atr_period: int = 14,
    ) -> None:
        self.name = "STAKE"
        self.min_apy = min_apy
        self.min_tvl_usd = min_tvl_usd
        self.max_risk_score = max_risk_score
        self.rotation_threshold_pct = rotation_threshold_pct
        self.atr_period = atr_period
        self._current_chain: Optional[str] = None
        self._current_apy: float = 0.0

    def generate_signal(
        self,
        daily_df=None,
        intraday_df=None,
        staking_yields: Optional[list] = None,
        **kwargs,
    ) -> Optional[dict]:
        if daily_df is None or len(daily_df) < 14:
            return None
        if not staking_yields:
            return None

        price = float(daily_df["close"].iloc[-1])
        atr = self._calc_atr(daily_df)
        if not np.isfinite(atr) or atr <= 0 or price <= 0:
            return None

        # Filter eligible protocols
        eligible = [
            p for p in staking_yields
            if isinstance(p, dict)
            and float(p.get("apy", 0)) >= self.min_apy
            and float(p.get("tvl_usd", 0)) >= self.min_tvl_usd
            and float(p.get("risk_score", 1.0)) <= self.max_risk_score
        ]
        if not eligible:
            return None

        # Risk-adjusted rank: APY / risk_score (higher is better)
        def sharpe_proxy(p: dict) -> float:
            risk = max(float(p.get("risk_score", 0.5)), 0.01)
            return float(p.get("apy", 0)) / risk

        best = max(eligible, key=sharpe_proxy)
        best_apy = float(best.get("apy", 0))
        best_chain = str(best.get("chain", "unknown"))
        best_risk = float(best.get("risk_score", 0.5))
        best_tvl = float(best.get("tvl_usd", 0))

        # Only signal if meaningfully better than current position
        improvement = best_apy - self._current_apy
        if improvement < self.rotation_threshold_pct and self._current_chain == best_chain:
            return None

        confidence = min(0.88, 0.55 + sharpe_proxy(best) * 0.15 - best_risk * 0.3)
        action = "ROTATE" if self._current_chain and self._current_chain != best_chain else "STAKE"

        self._current_chain = best_chain
        self._current_apy = best_apy

        return {
            "agent": self.name,
            "side": "BUY",
            "direction": "STAKE",
            "action": action,
            "entry": float(price),
            "stop": float(price - 1.5 * atr),
            "target": float(price + 2.0 * atr),
            "risk": float(1.5 * atr),
            "confidence": float(confidence),
            "reason": (
                f"STAKE: {action} → {best_chain} APY={best_apy:.1%} "
                f"risk={best_risk:.2f} TVL=${best_tvl/1e6:.0f}M "
                f"improvement={improvement:.1%}"
            ),
            "stake_meta": {
                "chain": best_chain,
                "apy": best_apy,
                "risk_score": best_risk,
                "tvl_usd": best_tvl,
                "sharpe_proxy": sharpe_proxy(best),
                "improvement_over_current": improvement,
                "action": action,
            },
            "strategy_version": "17.0.0",
        }

    def risk_profile(self) -> dict:
        return {
            "max_position": 1,
            "risk_per_trade": 0.002,
            "preferred_regime": ["LOW_VOL", "SIDEWAYS", "MEAN_REVERTING"],
            "requires_external_data": "staking_yields",
        }

    def explain(self) -> str:
        return (
            "STAKE ranks staking protocols by risk-adjusted APY (APY / risk_score). "
            "Rotates capital to the highest Sharpe staking opportunity when "
            "improvement exceeds 1.5% APY and TVL exceeds $100M."
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
