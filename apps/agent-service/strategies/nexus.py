"""NEXUS — Meta-fusion agent (CAFÉ-RC ensemble).

Aggregates signals from all peer agents and emits a consensus signal
only when a quorum of high-confidence agents agree on direction.
Requires agent_signals dict mapping agent_name → raw signal dict.
"""
from __future__ import annotations
from typing import Optional
import numpy as np


class NEXUS:
    def __init__(
        self,
        quorum_count: int = 3,
        min_avg_confidence: float = 0.72,
        min_agents_available: int = 5,
    ) -> None:
        self.name = "NEXUS"
        self.quorum_count = quorum_count
        self.min_avg_confidence = min_avg_confidence
        self.min_agents_available = min_agents_available

    def generate_signal(
        self,
        daily_df=None,
        intraday_df=None,
        agent_signals: Optional[dict] = None,
    ) -> Optional[dict]:
        if daily_df is None or len(daily_df) < 15:
            return None
        if not agent_signals or len(agent_signals) < self.min_agents_available:
            return None

        valid_signals = [
            s for s in agent_signals.values()
            if s is not None
            and isinstance(s, dict)
            and s.get("side") in ("BUY", "SELL")
            and isinstance(s.get("confidence"), (int, float))
            and float(s["confidence"]) >= 0.60
        ]

        if len(valid_signals) < self.quorum_count:
            return None

        buy_signals = [s for s in valid_signals if s["side"] == "BUY"]
        sell_signals = [s for s in valid_signals if s["side"] == "SELL"]

        dominant, opposite = (
            (buy_signals, sell_signals)
            if len(buy_signals) >= len(sell_signals)
            else (sell_signals, buy_signals)
        )

        if len(dominant) < self.quorum_count:
            return None

        # Reject if opposition is too strong (conflicting signals)
        if len(opposite) >= len(dominant):
            return None

        confidences = [float(s["confidence"]) for s in dominant]
        avg_confidence = float(np.mean(confidences))
        if avg_confidence < self.min_avg_confidence:
            return None

        side = dominant[0]["side"]
        price = float(daily_df["close"].iloc[-1])
        atr = self._calc_atr(daily_df)
        if not np.isfinite(atr) or atr <= 0 or price <= 0:
            return None

        # Aggregate entry from dominant signals (confidence-weighted mean)
        entries = np.array([float(s.get("entry", price)) for s in dominant])
        weights = np.array(confidences)
        weighted_entry = float(np.average(entries, weights=weights))

        stop_mult = 1.8
        target_mult = 2.5
        if side == "BUY":
            stop = weighted_entry - stop_mult * atr
            target = weighted_entry + target_mult * atr
        else:
            stop = weighted_entry + stop_mult * atr
            target = weighted_entry - target_mult * atr

        # Meta-confidence: blend avg_confidence with quorum strength
        quorum_ratio = len(dominant) / max(1, len(valid_signals))
        meta_confidence = min(0.95, avg_confidence * 0.7 + quorum_ratio * 0.3)

        contributing = [s.get("agent", s.get("strategy_id", "?")) for s in dominant]

        return {
            "agent": self.name,
            "side": side,
            "direction": side,
            "entry": float(weighted_entry),
            "stop": float(stop),
            "target": float(target),
            "risk": float(stop_mult * atr),
            "confidence": float(meta_confidence),
            "reason": (
                f"NEXUS quorum {len(dominant)}/{len(valid_signals)} agree {side}; "
                f"avg_conf={avg_confidence:.2f}; "
                f"contributors={contributing}"
            ),
            "nexus_meta": {
                "quorum": len(dominant),
                "total_valid": len(valid_signals),
                "avg_confidence": avg_confidence,
                "contributing_agents": contributing,
            },
            "strategy_version": "17.0.0",
        }

    def risk_profile(self) -> dict:
        return {
            "max_position": 1,
            "risk_per_trade": 0.008,
            "preferred_regime": ["ALL"],
            "requires_external_data": "agent_signals",
        }

    def explain(self) -> str:
        return (
            "NEXUS is the meta-fusion layer: it waits for a quorum of peer agents "
            "to agree on direction, then emits a single confidence-weighted signal "
            "with tighter stops and higher conviction."
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
