"""PHANTOM — CEX/DEX cross-venue arbitrage agent.

Detects persistent price dislocations between centralised exchanges (CEX)
and decentralised exchanges (DEX). Accounts for gas costs, bridge fees,
and execution risk when computing net edge.

Requires venue_books with at least one CEX and one DEX entry.
DEX prices should include expected slippage at the trade size.
"""
from __future__ import annotations
from typing import Optional
import numpy as np


class PHANTOM:
    def __init__(
        self,
        min_net_edge_pct: float = 0.005,  # 0.5% minimum after costs
        gas_cost_usd: float = 15.0,
        bridge_fee_pct: float = 0.001,
        cex_fee_pct: float = 0.001,
        dex_fee_pct: float = 0.003,
        max_notional_usd: float = 25_000.0,
    ) -> None:
        self.name = "PHANTOM"
        self.min_net_edge_pct = min_net_edge_pct
        self.gas_cost_usd = gas_cost_usd
        self.bridge_fee_pct = bridge_fee_pct
        self.cex_fee_pct = cex_fee_pct
        self.dex_fee_pct = dex_fee_pct
        self.max_notional_usd = max_notional_usd
        self._cex_venues = {"Binance", "Kraken", "Coinbase", "OKX"}
        self._dex_venues = {"Uniswap", "Curve", "1inch", "dYdX"}
        self._position_open = False

    def generate_signal(
        self,
        daily_df=None,
        intraday_df=None,
        venue_books: Optional[dict] = None,
        **kwargs,
    ) -> Optional[dict]:
        if daily_df is None or len(daily_df) < 2:
            return None
        if not venue_books or len(venue_books) < 2:
            return None
        if self._position_open:
            return None

        # Separate CEX and DEX books
        cex_books = {k: v for k, v in venue_books.items() if k in self._cex_venues}
        dex_books = {k: v for k, v in venue_books.items() if k in self._dex_venues}

        if not cex_books or not dex_books:
            return None

        # Find best CEX ask (cheapest to buy) and best DEX bid (highest to sell)
        best_cex_ask = min(
            (float(b["ask"]) for b in cex_books.values() if b.get("ask") and float(b["ask"]) > 0),
            default=None,
        )
        best_dex_bid = max(
            (float(b["bid"]) for b in dex_books.values() if b.get("bid") and float(b["bid"]) > 0),
            default=None,
        )

        if best_cex_ask is None or best_dex_bid is None:
            return None

        mid = (best_cex_ask + best_dex_bid) / 2.0
        if mid <= 0:
            return None

        # Gross edge: buy CEX, sell DEX
        gross_edge = (best_dex_bid - best_cex_ask) / best_cex_ask
        if gross_edge <= 0:
            return None

        # Cost model: gas + bridge + both-leg fees + slippage buffer
        notional = min(self.max_notional_usd, mid * 0.5)
        gas_pct = self.gas_cost_usd / notional
        total_cost = (
            self.cex_fee_pct
            + self.dex_fee_pct
            + self.bridge_fee_pct
            + gas_pct
            + 0.001  # slippage buffer
        )
        net_edge = gross_edge - total_cost

        if net_edge < self.min_net_edge_pct:
            return None

        confidence = min(0.90, 0.60 + net_edge * 20.0)
        price = float(daily_df["close"].iloc[-1])
        atr = self._calc_atr(daily_df)
        if not np.isfinite(atr) or atr <= 0:
            atr = price * 0.01

        best_cex_venue = min(cex_books, key=lambda k: float(cex_books[k].get("ask", 1e9)))
        best_dex_venue = max(dex_books, key=lambda k: float(dex_books[k].get("bid", 0)))

        self._position_open = True
        return {
            "agent": self.name,
            "side": "BUY",
            "direction": "CEX_DEX_ARB",
            "entry": float(best_cex_ask),
            "stop": float(best_cex_ask * (1 - net_edge * 0.5)),
            "target": float(best_dex_bid),
            "risk": float(atr * 0.5),
            "confidence": float(confidence),
            "reason": (
                f"PHANTOM: buy {best_cex_venue}@{best_cex_ask:.4f} "
                f"sell {best_dex_venue}@{best_dex_bid:.4f} "
                f"gross={gross_edge:.2%} net={net_edge:.2%}"
            ),
            "phantom_meta": {
                "cex_venue": best_cex_venue,
                "dex_venue": best_dex_venue,
                "cex_ask": best_cex_ask,
                "dex_bid": best_dex_bid,
                "gross_edge_pct": gross_edge,
                "net_edge_pct": net_edge,
                "notional_usd": notional,
            },
            "strategy_version": "17.0.0",
        }

    def on_fill(self) -> None:
        self._position_open = False

    def risk_profile(self) -> dict:
        return {
            "max_position": 1,
            "risk_per_trade": 0.003,
            "preferred_regime": ["ALL"],
            "requires_external_data": "venue_books (cex + dex)",
            "requires_bridge_capability": True,
        }

    def explain(self) -> str:
        return (
            "PHANTOM identifies CEX/DEX price dislocations exceeding 0.5% net of "
            "gas, bridge, and trading fees. Buys cheap on the CEX leg and sells "
            "higher on the DEX leg within the same atomic or near-atomic window."
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
