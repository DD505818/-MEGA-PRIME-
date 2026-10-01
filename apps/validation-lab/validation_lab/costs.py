"""After-cost accounting. Every number downstream is net of costs, or it is fiction.

Cost model (per bar):
  turnover      = |position[t] - position[t-1]|   (position in {-1, 0, 1})
  trade_cost    = turnover * (fee_bps + spread_slippage_bps) / 1e4
  funding_cost  = |position[t]| * funding_annual_bps / 1e4 / periods_per_year
  net_return[t] = gross_return[t] - trade_cost - funding_cost

cost_stress re-runs at 1x / 2x / 3x cost multiples: an edge that only survives
at exactly-quoted costs is not an edge.
"""

import numpy as np
import pandas as pd

from .metrics import sharpe, total_return

# Version of the after-cost accounting model. Bumped only when the formulas
# above change; persisted in every report's lineage block.
COST_MODEL_VERSION = "1.0.0"


def apply_costs(gross_returns, positions, fee_bps, spread_bps,
                funding_annual_bps=0.0, periods_per_year=24 * 365,
                cost_multiplier=1.0):
    gross = pd.Series(np.asarray(gross_returns, dtype=float)).reset_index(drop=True)
    pos = pd.Series(np.asarray(positions, dtype=float)).reset_index(drop=True)
    if len(gross) != len(pos):
        raise ValueError("gross_returns and positions must have equal length")
    pos = pos.fillna(0.0).clip(-1, 1)
    turnover = pos.diff().abs().fillna(pos.abs())
    trade_cost = turnover * (fee_bps + spread_bps) / 1e4 * cost_multiplier
    funding_cost = pos.abs() * funding_annual_bps / 1e4 / periods_per_year * cost_multiplier
    net = gross - trade_cost - funding_cost
    return {
        "net_returns": net,
        "gross_total": float(total_return(gross)),
        "net_total": float(total_return(net)),
        "total_trade_cost": float(trade_cost.sum()),
        "total_funding_cost": float(funding_cost.sum()),
        "total_cost": float(trade_cost.sum() + funding_cost.sum()),
        "n_trades": int((turnover > 0).sum()),
        "net_sharpe": sharpe(net, periods_per_year),
        "gross_sharpe": sharpe(gross, periods_per_year),
        "cost_multiplier": cost_multiplier,
    }


def cost_stress(gross_returns, positions, fee_bps, spread_bps,
                funding_annual_bps=0.0, periods_per_year=24 * 365,
                multipliers=(1.0, 2.0, 3.0)):
    out = {}
    for m in multipliers:
        r = apply_costs(gross_returns, positions, fee_bps, spread_bps,
                        funding_annual_bps, periods_per_year, m)
        out[f"{m:g}x"] = {
            "net_total_return": r["net_total"],
            "net_sharpe": r["net_sharpe"],
            "total_cost": r["total_cost"],
            "n_trades": r["n_trades"],
        }
    return out
