import time
from typing import Dict, Any, Optional

STAKING_ASSETS = {"ETH/USD", "SOL/USD", "DOT/USD", "AVAX/USD"}

def get_yields() -> Dict[str, Any]:
    return {
        "ETH": {"apy": 0.042, "lock": "none"},
        "SOL": {"apy": 0.068, "lock": "none"},
        "DOT": {"apy": 0.145, "lock": "28d"},
        "AVAX": {"apy": 0.089, "lock": "none"},
    }

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    symbol = payload.get("symbol", "")
    if symbol not in STAKING_ASSETS:
        return None
    price = payload.get("price", 0.0)
    if not price:
        return None
    asset = symbol.split("/")[0]
    yields = get_yields()
    apy = yields.get(asset, {}).get("apy", 0.0)
    return {
        "agent": "stake", "symbol": symbol, "side": "BUY",
        "action": "stake", "entry": round(price, 4), "apy": apy,
        "risk": 0.002, "confidence": 70.0, "ts": time.time(),
    }
