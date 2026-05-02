"""ARB — cross-exchange spread arbitrage agent.

Requires order_book_data with bid/ask from at least two venues.
Detects persistent mispricing above the combined round-trip fee threshold
and emits simultaneous buy-low/sell-high signal pairs.
"""
from __future__ import annotations
from typing import Optional
import numpy as np


class ARB:
    def __init__(
        self,
        min_edge_bps: float = 8.0,
        max_inventory_usd: float = 50_000.0,
        fee_bps: float = 3.0,
    ) -> None:
        self.name = "ARB"
        self.min_edge_bps = min_edge_bps
        self.max_inventory_usd = max_inventory_usd
        self.fee_bps = fee_bps  # one-way taker fee per leg
        self._position_open = False

    def generate_signal(
        self,
        daily_df=None,
        intraday_df=None,
        venue_books: Optional[dict] = None,
    ) -> Optional[dict]:
        if daily_df is None or len(daily_df) < 2:
            return None
        if not venue_books or len(venue_books) < 2:
            return None

        venues = list(venue_books.keys())
        best_bid_venue, best_ask_venue = None, None
        best_bid, best_ask = -np.inf, np.inf

        for venue, book in venue_books.items():
            bid = book.get("bid")
            ask = book.get("ask")
            if bid is None or ask is None or bid <= 0 or ask <= 0:
                continue
            if bid > best_bid:
                best_bid = bid
                best_bid_venue = venue
            if ask < best_ask:
                best_ask = ask
                best_ask_venue = venue

        if best_bid_venue is None or best_ask_venue is None:
            return None
        if best_bid_venue == best_ask_venue:
            return None  # no cross-venue opportunity
        if best_ask >= best_bid:
            return None  # no raw spread

        mid = (best_bid + best_ask) / 2.0
        if mid <= 0:
            return None

        gross_edge_bps = ((best_bid - best_ask) / mid) * 10_000.0
        round_trip_cost_bps = 2 * self.fee_bps
        net_edge_bps = gross_edge_bps - round_trip_cost_bps

        if net_edge_bps < self.min_edge_bps:
            return None
        if self._position_open:
            return None

        price = float(daily_df["close"].iloc[-1])
        atr = self._calc_atr(daily_df)
        if not np.isfinite(atr) or atr <= 0:
            return None

        notional = min(self.max_inventory_usd, price * 0.1)
        confidence = min(0.92, 0.60 + net_edge_bps / 100.0)

        self._position_open = True
        return {
            "agent": self.name,
            "side": "BUY",
            "direction": "ARB",
            "entry": float(best_ask),
            "stop": float(best_ask - 1.0 * atr),
            "target": float(best_bid),
            "risk": float(atr),
            "confidence": float(confidence),
            "reason": (
                f"Cross-venue arb: buy {best_ask_venue}@{best_ask:.2f} "
                f"sell {best_bid_venue}@{best_bid:.2f} "
                f"net_edge={net_edge_bps:.1f}bps"
            ),
            "arb_meta": {
                "buy_venue": best_ask_venue,
                "sell_venue": best_bid_venue,
                "buy_price": float(best_ask),
                "sell_price": float(best_bid),
                "net_edge_bps": float(net_edge_bps),
                "notional_usd": float(notional),
            },
            "strategy_version": "17.0.0",
        }

    def on_fill(self) -> None:
        self._position_open = False

    def risk_profile(self) -> dict:
        return {
            "max_position": 1,
            "risk_per_trade": 0.002,
            "preferred_regime": ["ALL"],
            "requires_external_data": "venue_books",
            "max_notional_usd": self.max_inventory_usd,
        }

    def explain(self) -> str:
        return (
            "ARB detects cross-venue price dislocations exceeding the combined "
            "round-trip fee threshold and emits simultaneous buy/sell leg signals."
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
