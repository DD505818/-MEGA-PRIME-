import time
from typing import Dict, Any, Optional

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    corr = payload.get("correlation", 0.0)
    vol  = payload.get("volatility", 0.0)
    price = payload.get("close", 0.0)
    if not price or abs(corr) < 0.7:
        return None
    side = "BUY" if corr > 0 and vol > 0.01 else "SELL" if corr < 0 else None
    if not side:
        return None
    mult = 1 if side == "BUY" else -1
    return {
        "agent": "nexus", "symbol": payload.get("symbol", "BTC/USD"),
        "side": side, "entry": round(price, 4),
        "stop": round(price * (1 - mult * 0.007), 4),
        "tp1": round(price * (1 + mult * 0.014), 4),
        "tp2": round(price * (1 + mult * 0.028), 4),
        "correlation": round(corr, 3),
        "risk": 0.005, "confidence": round(abs(corr) * 90, 1), "ts": time.time(),
    }
