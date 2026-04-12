import time
from typing import Dict, Any, Optional

MEV_THRESHOLD = 0.002   # min 0.2% profit opportunity

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    mempool_price = payload.get("mempool_price", 0.0)
    market_price  = payload.get("price", 0.0)
    if not mempool_price or not market_price:
        return None
    spread = abs(mempool_price - market_price) / market_price
    if spread < MEV_THRESHOLD:
        return None
    side = "BUY" if mempool_price > market_price else "SELL"
    mult = 1 if side == "BUY" else -1
    return {
        "agent": "mev-hunter", "symbol": payload.get("symbol", "ETH/USD"),
        "side": side, "entry": round(market_price, 4),
        "target": round(mempool_price, 4),
        "stop": round(market_price * (1 - mult * 0.005), 4),
        "tp1": round(mempool_price, 4), "tp2": round(mempool_price * (1 + mult * 0.003), 4),
        "mev_spread_pct": round(spread * 100, 4),
        "risk": 0.003, "confidence": round(min(spread * 3000, 85), 1),
        "ts": time.time(),
    }
