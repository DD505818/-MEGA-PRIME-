"""ORBIT — macro regime rotation agent.

Rotates across asset classes (equities, bonds, commodities, cash) based
on PMI, interest rate levels, credit spreads, and yield curve slope.

Signal hierarchy:
- PMI > 52 + rates < 3%   → overweight equities
- PMI 48–52               → neutral / diversified
- PMI < 48                → overweight bonds / defensive
- Inverted yield curve     → risk-off, shift to gold + short duration
- Credit spreads > 300bps  → risk-off signal

Requires macro_data dict with PMI, rates, credit_spread, yield_curve_slope.
"""
from __future__ import annotations
from typing import Optional
import numpy as np


class ORBIT:
    def __init__(
        self,
        pmi_bull_threshold: float = 52.0,
        pmi_bear_threshold: float = 48.0,
        rate_bull_ceiling: float = 3.5,
        credit_spread_risk_off: float = 300.0,
        yield_curve_inversion: float = -0.1,
        atr_period: int = 14,
    ) -> None:
        self.name = "ORBIT"
        self.pmi_bull_threshold = pmi_bull_threshold
        self.pmi_bear_threshold = pmi_bear_threshold
        self.rate_bull_ceiling = rate_bull_ceiling
        self.credit_spread_risk_off = credit_spread_risk_off
        self.yield_curve_inversion = yield_curve_inversion
        self.atr_period = atr_period

    def generate_signal(
        self,
        daily_df=None,
        intraday_df=None,
        macro_data: Optional[dict] = None,
        **kwargs,
    ) -> Optional[dict]:
        if daily_df is None or len(daily_df) < 20:
            return None
        if not macro_data:
            return None

        pmi = float(macro_data.get("pmi", 50.0))
        rates = float(macro_data.get("rates", 4.0))
        credit_spread = float(macro_data.get("credit_spread_bps", 150.0))
        yield_curve = float(macro_data.get("yield_curve_slope", 0.5))

        price = float(daily_df["close"].iloc[-1])
        atr = self._calc_atr(daily_df)
        if not np.isfinite(atr) or atr <= 0 or price <= 0:
            return None

        # ── Crisis / risk-off conditions ─────────────────────────────────────
        if yield_curve < self.yield_curve_inversion or credit_spread > self.credit_spread_risk_off:
            return {
                "agent": self.name,
                "side": "SELL",
                "direction": "MACRO_RISK_OFF",
                "regime": "RISK_OFF",
                "entry": float(price),
                "stop": float(price + 2.5 * atr),
                "target": float(price - 3.0 * atr),
                "risk": float(2.5 * atr),
                "confidence": 0.82,
                "reason": (
                    f"ORBIT: risk-off — "
                    f"yield_curve={yield_curve:.2f} "
                    f"credit_spread={credit_spread:.0f}bps"
                ),
                "orbit_meta": self._meta(pmi, rates, credit_spread, yield_curve),
                "strategy_version": "17.0.0",
            }

        # ── Bull regime: PMI expanding + accommodative rates ─────────────────
        if pmi > self.pmi_bull_threshold and rates <= self.rate_bull_ceiling:
            pmi_strength = (pmi - self.pmi_bull_threshold) / 5.0
            confidence = min(0.86, 0.62 + pmi_strength * 0.08)
            return {
                "agent": self.name,
                "side": "BUY",
                "direction": "MACRO_RISK_ON",
                "regime": "RISK_ON",
                "entry": float(price),
                "stop": float(price - 2.0 * atr),
                "target": float(price + 2.8 * atr),
                "risk": float(2.0 * atr),
                "confidence": float(confidence),
                "reason": (
                    f"ORBIT: risk-on — PMI={pmi:.1f} rates={rates:.2f}% "
                    f"spread={credit_spread:.0f}bps"
                ),
                "orbit_meta": self._meta(pmi, rates, credit_spread, yield_curve),
                "strategy_version": "17.0.0",
            }

        # ── Bear regime: PMI contracting ─────────────────────────────────────
        if pmi < self.pmi_bear_threshold:
            pmi_weakness = (self.pmi_bear_threshold - pmi) / 5.0
            confidence = min(0.83, 0.60 + pmi_weakness * 0.08)
            return {
                "agent": self.name,
                "side": "SELL",
                "direction": "MACRO_DEFENSIVE",
                "regime": "DEFENSIVE",
                "entry": float(price),
                "stop": float(price + 2.0 * atr),
                "target": float(price - 2.5 * atr),
                "risk": float(2.0 * atr),
                "confidence": float(confidence),
                "reason": (
                    f"ORBIT: defensive — PMI={pmi:.1f} contracting, "
                    f"rates={rates:.2f}%"
                ),
                "orbit_meta": self._meta(pmi, rates, credit_spread, yield_curve),
                "strategy_version": "17.0.0",
            }

        return None

    def _meta(self, pmi, rates, credit_spread, yield_curve) -> dict:
        return {
            "pmi": pmi,
            "rates_pct": rates,
            "credit_spread_bps": credit_spread,
            "yield_curve_slope": yield_curve,
        }

    def risk_profile(self) -> dict:
        return {
            "max_position": 1,
            "risk_per_trade": 0.006,
            "preferred_regime": ["ALL"],
            "requires_external_data": "macro_data",
        }

    def explain(self) -> str:
        return (
            "ORBIT rotates asset class exposure based on the macro cycle: "
            "PMI expansion + low rates → risk-on equities; "
            "PMI contraction → defensive bonds/gold; "
            "inverted curve or wide credit spreads → full risk-off."
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
