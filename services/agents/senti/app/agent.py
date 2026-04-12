import time
from typing import Dict, Any, Optional

_last_sentiment = {"score": 0.5, "label": "neutral", "ts": 0.0}

def get_sentiment() -> Dict[str, Any]:
    return _last_sentiment

def generate_signal(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    global _last_sentiment
    score = payload.get("sentiment_score", 0.5)
    label = "bullish" if score > 0.65 else "bearish" if score < 0.35 else "neutral"
    _last_sentiment = {"score": score, "label": label, "ts": time.time()}
    if label == "neutral":
        return None
    price = payload.get("price", 0.0)
    if not price:
        return None
    side = "BUY" if label == "bullish" else "SELL"
    mult = 1 if side == "BUY" else -1
    return {
        "agent": "senti", "symbol": payload.get("symbol", "BTC/USD"),
        "side": side, "entry": round(price, 4),
        "stop": round(price * (1 - mult * 0.009), 4),
        "tp1": round(price * (1 + mult * 0.015), 4),
        "tp2": round(price * (1 + mult * 0.030), 4),
        "sentiment": label, "score": round(score, 3),
        "risk": 0.004, "confidence": round(abs(score - 0.5) * 180, 1), "ts": time.time(),
    }
