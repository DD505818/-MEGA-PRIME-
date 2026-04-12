import time
from typing import Dict, Any, Optional

SYMBOLS = ["BTC/USD", "ETH/USD", "SOL/USD"]
RISK    = 0.005

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    vol = payload.get("volatility", 0.0)
    price = payload.get("close", payload.get("price", 0.0))
    if not price or vol < 0.01:
        return None
    # Box Theory: sweep below PDL then re-enter → long bias
    pdl = payload.get("pdl", price * 0.995)
    pdh = payload.get("pdh", price * 1.005)
    mid = (pdh + pdl) / 2
    symbol = payload.get("symbol", SYMBOLS[0])

    if price < pdl * 1.001:           # sweep + re-entry
        side = "BUY"
        entry = price
        stop  = pdl * 0.998
        tp1   = mid
        tp2   = pdh
    elif price > pdh * 0.999:
        side = "SELL"
        entry = price
        stop  = pdh * 1.002
        tp1   = mid
        tp2   = pdl
    else:
        return None

    return {
        "agent": "mamba", "symbol": symbol, "side": side,
        "entry": round(entry, 4), "stop": round(stop, 4),
        "tp1": round(tp1, 4), "tp2": round(tp2, 4),
        "risk": RISK, "confidence": round(min(vol * 40, 95), 1),
        "ts": time.time(),
    }
