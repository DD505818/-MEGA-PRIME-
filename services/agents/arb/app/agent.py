import time
from typing import Dict, Any, Optional

SPREAD_THRESH = 0.001   # 0.1% minimum

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    bid = payload.get("bid", 0.0)
    ask = payload.get("ask", 0.0)
    if not bid or not ask:
        return None
    spread = (ask - bid) / bid
    if spread < SPREAD_THRESH:
        return None
    return {
        "agent": "arb", "symbol": payload.get("symbol", "BTC/USD"),
        "side": "BUY", "entry": round(bid, 4),
        "stop": round(bid * 0.997, 4),
        "tp1": round(ask, 4), "tp2": round(ask * 1.003, 4),
        "spread_pct": round(spread * 100, 4),
        "risk": 0.005, "confidence": round(min(spread * 5000, 85), 1),
        "ts": time.time(),
    }
