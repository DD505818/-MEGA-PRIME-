import time
from typing import Dict, Any, Optional

def _classify_regime(vol: float, trend: float) -> str:
    if vol > 0.035:   return "high_vol"
    if trend > 0.5:   return "trend"
    return "mean_reverting"

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    vol   = payload.get("volatility", 0.0)
    trend = payload.get("trend", 0.0)
    price = payload.get("close", 0.0)
    if not price:
        return None
    regime = _classify_regime(vol, trend)
    if regime == "high_vol":
        return None   # no trades in extreme vol
    side = "BUY" if trend > 0 else "SELL"
    mult = 1 if side == "BUY" else -1
    return {
        "agent": "orbit", "symbol": payload.get("symbol", "BTC/USD"),
        "side": side, "regime": regime, "entry": round(price, 4),
        "stop": round(price * (1 - mult * 0.007), 4),
        "tp1": round(price * (1 + mult * 0.012), 4),
        "tp2": round(price * (1 + mult * 0.024), 4),
        "risk": 0.005, "confidence": 72.0, "ts": time.time(),
    }
