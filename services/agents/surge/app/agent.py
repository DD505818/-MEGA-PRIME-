import time
from typing import Dict, Any, Optional

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    vol = payload.get("volatility", 0.0)
    price = payload.get("close", payload.get("price", 0.0))
    if not price or vol < 0.025:
        return None
    symbol = payload.get("symbol", "BTC/USD")
    side = "BUY" if payload.get("trend", 0) >= 0 else "SELL"
    mult = 1 if side == "BUY" else -1
    return {
        "agent": "surge", "symbol": symbol, "side": side,
        "entry": round(price, 4),
        "stop": round(price * (1 - mult * 0.008), 4),
        "tp1": round(price * (1 + mult * 0.012), 4),
        "tp2": round(price * (1 + mult * 0.024), 4),
        "risk": 0.005, "confidence": round(min(vol * 30, 90), 1),
        "ts": time.time(),
    }
