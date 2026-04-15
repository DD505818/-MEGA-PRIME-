"""Run a deterministic toy backtest across sampled prices."""

from __future__ import annotations

import json
import sys
from pathlib import Path

PROJECT_ROOT = Path(__file__).resolve().parents[1]
if str(PROJECT_ROOT) not in sys.path:
    sys.path.insert(0, str(PROJECT_ROOT))

from ml.impact_model.model import estimate_slippage
from ml.price_predictor.transformer_model import predict_next


def run_backtest() -> dict[str, float]:
    prices = [42000, 42120, 42080, 42210, 42300, 42420, 42390]
    forecast = predict_next(prices)
    edge = (forecast - prices[-1]) / prices[-1]

    qty = 5.0 if edge > 0 else 2.0
    slip = estimate_slippage(qty=qty, volatility=0.02, liquidity=5000)
    gross_return = edge * qty
    net_return = gross_return - slip

    return {
        "last_price": prices[-1],
        "forecast": forecast,
        "edge": round(edge, 6),
        "qty": qty,
        "slippage": slip,
        "net_return": round(net_return, 6),
    }


if __name__ == "__main__":
    result = run_backtest()
    artifacts = PROJECT_ROOT / "artifacts"
    artifacts.mkdir(parents=True, exist_ok=True)
    out = artifacts / "backtest-result.json"
    out.write_text(json.dumps(result, indent=2), encoding="utf-8")
    print(json.dumps(result, indent=2))
