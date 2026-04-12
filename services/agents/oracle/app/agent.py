import time, math
from typing import Dict, Any, Optional

_forecast_cache: Dict[str, Any] = {}

def get_forecast() -> Dict[str, Any]:
    return _forecast_cache

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    price = payload.get("close", payload.get("price", 0.0))
    vol   = payload.get("volatility", 0.0)
    if not price:
        return None
    # Simple exponential smoothing forecast
    alpha = 0.3
    prev  = _forecast_cache.get("price", price)
    forecast = alpha * price + (1 - alpha) * prev
    _forecast_cache.update({"price": forecast, "actual": price, "ts": time.time()})

    delta = (forecast - price) / price
    if abs(delta) < 0.003:
        return None
    side = "BUY" if delta > 0 else "SELL"
    mult = 1 if side == "BUY" else -1
    return {
        "agent": "oracle", "symbol": payload.get("symbol", "BTC/USD"),
        "side": side, "entry": round(price, 4), "forecast": round(forecast, 4),
        "stop": round(price * (1 - mult * 0.008), 4),
        "tp1": round(forecast, 4),
        "tp2": round(price * (1 + mult * 0.025), 4),
        "risk": 0.005, "confidence": round(min(abs(delta) * 2000, 88), 1),
        "ts": time.time(),
    }
