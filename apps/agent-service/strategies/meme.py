"""MEME — social-momentum burst trading agent.

Detects parabolic social-volume spikes in high-volatility micro-cap assets.
Uses a composite momentum score combining:
- Social mention velocity (7-day vs 30-day average)
- Price momentum (current vs 20-day SMA)
- Volume spike factor (current vs 20-day average volume)
- Sentiment polarity (net positive mentions ratio)

RISK WARNING: Paper mode strongly recommended. Position size capped at 0.5%
of portfolio. Strict stop at 1.5× ATR to prevent runaway losses.

Requires meme_data: list of dicts per coin.
"""
from __future__ import annotations
from typing import Optional
import numpy as np


class MEME:
    def __init__(
        self,
        min_social_spike: float = 2.5,
        min_volume_spike: float = 3.0,
        min_price_momentum: float = 0.05,
        min_sentiment_ratio: float = 0.55,
        min_mentions: int = 500,
        atr_stop_mult: float = 1.5,
        atr_period: int = 14,
    ) -> None:
        self.name = "MEME"
        self.min_social_spike = min_social_spike
        self.min_volume_spike = min_volume_spike
        self.min_price_momentum = min_price_momentum
        self.min_sentiment_ratio = min_sentiment_ratio
        self.min_mentions = min_mentions
        self.atr_stop_mult = atr_stop_mult
        self.atr_period = atr_period

    def generate_signal(
        self,
        daily_df=None,
        intraday_df=None,
        meme_data: Optional[list] = None,
        **kwargs,
    ) -> Optional[dict]:
        if daily_df is None or len(daily_df) < self.atr_period + 5:
            return None
        if not meme_data:
            return None

        price = float(daily_df["close"].iloc[-1])
        atr = self._calc_atr(daily_df)
        if not np.isfinite(atr) or atr <= 0 or price <= 0:
            return None

        best_coin = self._rank_candidates(meme_data)
        if best_coin is None:
            return None

        symbol = str(best_coin.get("symbol", "UNKNOWN"))
        momentum_score = float(best_coin.get("_momentum_score", 0.0))
        social_spike = float(best_coin.get("social_spike", 1.0))
        volume_spike = float(best_coin.get("volume_spike", 1.0))
        price_mom = float(best_coin.get("price_momentum", 0.0))
        sentiment = float(best_coin.get("sentiment_ratio", 0.5))
        mentions = int(best_coin.get("mentions_24h", 0))
        coin_price = float(best_coin.get("price", price))

        # Use coin_price if available, else fall back to primary df
        if coin_price > 0:
            entry = coin_price
        else:
            entry = price

        stop = entry * (1.0 - self.atr_stop_mult * (atr / price))
        target = entry * (1.0 + 2.5 * (atr / price))

        # Confidence capped at 0.78 — meme plays are inherently lower conviction
        confidence = min(0.78, 0.50 + momentum_score * 0.15)

        return {
            "agent": self.name,
            "side": "BUY",
            "direction": "MEME_MOMENTUM",
            "symbol": symbol,
            "entry": float(entry),
            "stop": float(stop),
            "target": float(target),
            "risk": float(entry - stop),
            "confidence": float(confidence),
            "reason": (
                f"MEME: {symbol} social_spike={social_spike:.1f}x "
                f"vol_spike={volume_spike:.1f}x "
                f"price_mom={price_mom:.1%} "
                f"sentiment={sentiment:.0%} "
                f"mentions={mentions:,}"
            ),
            "meme_meta": {
                "symbol": symbol,
                "social_spike": social_spike,
                "volume_spike": volume_spike,
                "price_momentum": price_mom,
                "sentiment_ratio": sentiment,
                "mentions_24h": mentions,
                "momentum_score": momentum_score,
            },
            "strategy_version": "17.0.0",
        }

    def _rank_candidates(self, meme_data: list) -> Optional[dict]:
        scored = []
        for coin in meme_data:
            if not isinstance(coin, dict):
                continue

            social_spike = float(coin.get("social_spike", 1.0))
            volume_spike = float(coin.get("volume_spike", 1.0))
            price_mom = float(coin.get("price_momentum", 0.0))
            sentiment = float(coin.get("sentiment_ratio", 0.5))
            mentions = int(coin.get("mentions_24h", 0))

            if social_spike < self.min_social_spike:
                continue
            if volume_spike < self.min_volume_spike:
                continue
            if price_mom < self.min_price_momentum:
                continue
            if sentiment < self.min_sentiment_ratio:
                continue
            if mentions < self.min_mentions:
                continue

            score = (
                np.log1p(social_spike) * 0.35
                + np.log1p(volume_spike) * 0.30
                + price_mom * 5.0 * 0.20
                + (sentiment - 0.5) * 2.0 * 0.15
            )
            coin["_momentum_score"] = float(score)
            scored.append(coin)

        if not scored:
            return None
        return max(scored, key=lambda c: c["_momentum_score"])

    def risk_profile(self) -> dict:
        return {
            "max_position": 1,
            "risk_per_trade": 0.003,
            "max_position_pct": 0.005,
            "preferred_regime": ["HIGH_VOL"],
            "requires_external_data": "meme_data",
            "paper_only_recommended": True,
        }

    def explain(self) -> str:
        return (
            "MEME surfs parabolic social-volume bursts: requires 2.5× social spike, "
            "3× volume spike, >5% price momentum, and >55% positive sentiment. "
            "Position capped at 0.5% of portfolio. Stop at 1.5× ATR."
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
