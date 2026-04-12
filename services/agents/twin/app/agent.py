import time
from typing import Dict, Any, Optional

PAIR_RATIO_THRESHOLD = 0.02

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    ratio = payload.get("pair_ratio", 1.0)
    mean  = payload.get("pair_mean", 1.0)
    if not mean:
        return None
    zscore = (ratio - mean) / max(payload.get("pair_std", 0.01), 1e-9)
    if abs(zscore) < 2.0:
        return None
    side = "SELL" if zscore > 0 else "BUY"
    return {
        "agent": "twin", "symbol": payload.get("symbol", "BTC/USD"),
        "pair": payload.get("pair", "ETH/USD"),
        "side": side, "zscore": round(zscore, 3),
        "risk": 0.005, "confidence": round(min(abs(zscore) * 25, 92), 1),
        "ts": time.time(),
    }
