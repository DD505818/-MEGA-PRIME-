import time
from typing import Dict, Any, Optional, List

PHI   = 1.6180339887
FIBS  = [0.236, 0.382, 0.5, 0.618, 0.786]

def _fib_levels(high: float, low: float) -> List[float]:
    rng = high - low
    return [round(high - rng * f, 4) for f in FIBS]

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    high  = payload.get("high", 0.0)
    low   = payload.get("low", 0.0)
    price = payload.get("close", payload.get("price", 0.0))
    if not all([high, low, price]) or high <= low:
        return None
    levels = _fib_levels(high, low)
    # Find nearest Fibonacci level
    nearest = min(levels, key=lambda l: abs(l - price))
    dist    = abs(nearest - price) / price
    if dist > 0.003:   # not near enough
        return None
    side = "BUY" if price <= nearest else "SELL"
    mult = 1 if side == "BUY" else -1
    return {
        "agent": "midas", "symbol": payload.get("symbol", "BTC/USD"),
        "side": side, "entry": round(price, 4),
        "fib_level": nearest, "fib_levels": levels,
        "stop": round(price * (1 - mult * 0.006), 4),
        "tp1": round(price * (1 + mult * 0.010), 4),
        "tp2": round(price * (1 + mult * PHI * 0.008), 4),
        "risk": 0.005, "confidence": round((1 - dist * 200) * 85, 1),
        "ts": time.time(),
    }
