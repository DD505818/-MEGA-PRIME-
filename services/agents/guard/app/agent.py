import time
from typing import Dict, Any, Optional

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    loss = payload.get("loss", 0.0)
    exposure = payload.get("exposure", 0.0)
    symbol = payload.get("symbol", "BTC/USD")
    price = payload.get("price", 0.0)
    if loss < 0.015 and exposure < 0.20:
        return None
    return {
        "agent": "guard", "symbol": symbol, "side": "SELL",
        "entry": round(price, 4) if price else 0,
        "stop": 0, "tp1": 0, "tp2": 0,
        "action": "hedge", "risk": 0.005,
        "confidence": 90.0, "ts": time.time(),
    }
