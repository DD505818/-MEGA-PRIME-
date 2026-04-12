import time, math
from typing import Dict, Any, Optional

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    imbalance = payload.get("order_imbalance", 0.0)
    price = payload.get("price", payload.get("close", 0.0))
    if not price or abs(imbalance) < 0.3:
        return None
    side = "BUY" if imbalance > 0 else "SELL"
    mult = 1 if side == "BUY" else -1
    conf = round(min(abs(imbalance) * 100, 95), 1)
    return {
        "agent": "opt", "symbol": payload.get("symbol", "BTC/USD"),
        "side": side, "entry": round(price, 4),
        "stop": round(price * (1 - mult * 0.006), 4),
        "tp1": round(price * (1 + mult * 0.010), 4),
        "tp2": round(price * (1 + mult * 0.020), 4),
        "risk": 0.005, "confidence": conf, "ts": time.time(),
    }
