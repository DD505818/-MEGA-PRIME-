import time
from typing import Dict, Any, Optional

GOLD_SYMBOLS = ["XAU/USD", "BTC/USD"]

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    symbol = payload.get("symbol", "")
    if symbol not in GOLD_SYMBOLS:
        return None
    price = payload.get("price", 0.0)
    if not price:
        return None
    change = payload.get("change_24h", 0.0)
    if change > 0.005:          # flight-to-quality inflow
        side = "BUY"
        mult = 1
    elif change < -0.015:
        side = "SELL"
        mult = -1
    else:
        return None
    return {
        "agent": "gold", "symbol": symbol, "side": side,
        "entry": round(price, 4),
        "stop": round(price * (1 - mult * 0.008), 4),
        "tp1": round(price * (1 + mult * 0.015), 4),
        "tp2": round(price * (1 + mult * 0.030), 4),
        "risk": 0.005, "confidence": 75.0, "ts": time.time(),
    }
