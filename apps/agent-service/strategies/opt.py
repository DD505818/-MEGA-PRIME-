"""OPT — Options Volatility Arbitrage agent.

Requires options_data with iv_rank, iv_hv_ratio, and optionally skew_data.
Trades volatility dislocations via short strangle/straddle candidates.
Immediately signals delta hedge after entry.

Config mirrors OPT_CONFIG from the ΩMEGA PRIME Δ strategy spec.
"""
from __future__ import annotations
from typing import Optional
import numpy as np


OPT_CONFIG = {
    "iv_sigma_threshold": 3.0,
    "iv_skew_arb_enabled": True,
    "pricing_discrepancy_min_pct": 0.15,
    "delta_hedge_immediate": True,
    "profit_target_pct": {"min": 0.5, "max": 2.0},
    "min_account_size": 100_000,
    "close_on_settlement": True,
    "close_on_vol_normalize": True,
}


class OPT:
    def __init__(
        self,
        iv_sigma_threshold: float = OPT_CONFIG["iv_sigma_threshold"],
        pricing_discrepancy_min_pct: float = OPT_CONFIG["pricing_discrepancy_min_pct"],
        max_trades_per_day: int = 2,
    ) -> None:
        self.name = "OPT"
        self.iv_sigma_threshold = iv_sigma_threshold
        self.pricing_discrepancy_min_pct = pricing_discrepancy_min_pct
        self.max_trades_per_day = max_trades_per_day
        self._trades_today = 0
        self._last_date: Optional[str] = None

    def generate_signal(
        self,
        daily_df=None,
        intraday_df=None,
        options_data: Optional[dict] = None,
    ) -> Optional[dict]:
        if daily_df is None or len(daily_df) < 30:
            return None
        if not options_data:
            return None

        iv_rank = options_data.get("iv_rank")
        iv = options_data.get("iv")
        hv = options_data.get("hv")
        iv_mean = options_data.get("iv_mean_30d")
        iv_std = options_data.get("iv_std_30d")

        if any(v is None for v in [iv_rank, iv, hv]):
            return None

        iv_rank = float(iv_rank)
        iv = float(iv)
        hv = float(hv)
        iv_hv_ratio = iv / hv if hv > 0 else 0.0

        # Reset daily trade counter
        import time
        today = time.strftime("%Y-%m-%d")
        if today != self._last_date:
            self._trades_today = 0
            self._last_date = today

        if self._trades_today >= self.max_trades_per_day:
            return None

        # IV must be > N sigma above 30-day mean
        if iv_mean is not None and iv_std is not None and iv_std > 0:
            iv_z = (iv - float(iv_mean)) / float(iv_std)
        else:
            iv_z = (iv_rank - 70.0) / 10.0  # fallback rank-based z-score

        if iv_z < self.iv_sigma_threshold:
            return None

        # IV/HV ratio check (legacy gate still useful)
        if iv_hv_ratio < 1.3:
            return None

        # Skew arbitrage check
        skew_data = options_data.get("skew_data", {})
        skew_discrepancy = float(skew_data.get("discrepancy_pct", 0.0))
        if OPT_CONFIG["iv_skew_arb_enabled"] and skew_discrepancy < self.pricing_discrepancy_min_pct:
            return None

        price = float(daily_df["close"].iloc[-1])
        atr = self._calc_atr(daily_df)
        if not np.isfinite(atr) or atr <= 0 or price <= 0:
            return None

        confidence = min(0.88, 0.55 + iv_z / 10.0 + (iv_hv_ratio - 1.3) / 5.0)
        self._trades_today += 1

        return {
            "agent": self.name,
            "side": "SELL",
            "direction": "SHORT_VOL",
            "entry": float(price),
            "stop": float(price + 2.5 * atr),
            "target": float(price - 1.0 * atr),
            "risk": float(2.5 * atr),
            "confidence": float(confidence),
            "reason": (
                f"IV elevated {iv_z:.1f}σ above 30d mean; "
                f"IV/HV={iv_hv_ratio:.2f}; "
                f"skew_discrepancy={skew_discrepancy:.1%}; "
                f"short vol via strangle"
            ),
            "options_meta": {
                "iv": iv,
                "hv": hv,
                "iv_hv_ratio": iv_hv_ratio,
                "iv_rank": iv_rank,
                "iv_sigma_z": float(iv_z),
                "delta_hedge_required": OPT_CONFIG["delta_hedge_immediate"],
                "close_on_settlement": OPT_CONFIG["close_on_settlement"],
                "close_on_vol_normalize": OPT_CONFIG["close_on_vol_normalize"],
            },
            "strategy_version": "17.0.0",
        }

    def risk_profile(self) -> dict:
        return {
            "max_position": 2,
            "risk_per_trade": 0.02,
            "preferred_regime": ["HIGH_VOL", "MEAN_REVERTING"],
            "requires_external_data": "options_data",
            "min_account_size": OPT_CONFIG["min_account_size"],
        }

    def explain(self) -> str:
        return (
            "OPT sells volatility when IV is more than 3σ above its 30-day mean "
            "and the IV/HV ratio exceeds 1.3. Delta is hedged immediately. "
            "Positions close at daily settlement or when vol normalizes."
        )

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
