import time
from typing import Dict, Any, Optional

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    apy = payload.get("apy", 0.0)
    price = payload.get("price", 0.0)
    if not price or apy < 0.05:   # minimum 5% APY
        return None
    return {
        "agent": "harvest", "symbol": payload.get("symbol", "ETH/USD"),
        "side": "BUY", "action": "yield_enter",
        "entry": round(price, 4), "apy": round(apy, 4),
        "stop": round(price * 0.92, 4), "tp1": round(price * 1.05, 4),
        "risk": 0.003, "confidence": round(min(apy * 500, 80), 1), "ts": time.time(),
    }
