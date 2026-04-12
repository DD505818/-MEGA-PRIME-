import time
from typing import Dict, Any, Optional

SPREAD = 0.0004   # 4bps

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    price = payload.get("price", payload.get("mid", 0.0))
    if not price:
        return None
    bid = price * (1 - SPREAD / 2)
    ask = price * (1 + SPREAD / 2)
    return {
        "agent": "maker", "symbol": payload.get("symbol", "BTC/USD"),
        "side": "BOTH", "bid": round(bid, 4), "ask": round(ask, 4),
        "spread_bps": round(SPREAD * 10000, 2),
        "risk": 0.003, "confidence": 80.0, "ts": time.time(),
    }
