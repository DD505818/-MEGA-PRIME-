import time
from typing import Dict, Any, Optional

SCALP_TARGET = 0.0015   # 0.15%
SCALP_STOP   = 0.0008   # 0.08%

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    price  = payload.get("price", 0.0)
    volume = payload.get("volume", 0.0)
    change = payload.get("change_1m", 0.0)
    if not price or volume < 100:
        return None
    if abs(change) < 0.0003:   # too quiet for scalping
        return None
    side = "BUY" if change > 0 else "SELL"
    mult = 1 if side == "BUY" else -1
    return {
        "agent": "hi-darts", "symbol": payload.get("symbol", "BTC/USD"),
        "side": side, "entry": round(price, 4),
        "stop": round(price * (1 - mult * SCALP_STOP), 4),
        "tp1": round(price * (1 + mult * SCALP_TARGET), 4),
        "tp2": round(price * (1 + mult * SCALP_TARGET * 2), 4),
        "risk": 0.002, "confidence": round(min(abs(change) * 10000 * 3, 80), 1),
        "ts": time.time(),
    }
