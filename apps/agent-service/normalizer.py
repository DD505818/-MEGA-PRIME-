import uuid, time, os

def normalize_signal(strategy_output, strategy_name):
    if not strategy_output:
        return None
    qty = 0.01
    return {
        "signal_id": str(uuid.uuid4()),
        "strategy_id": strategy_name,
        "symbol": "BTCUSDT",
        "side": strategy_output.get("side", "BUY"),
        "quantity": qty,
        "limit_price": strategy_output.get("entry", 0),
        "confidence": strategy_output.get("confidence", 0.7),
        "timestamp": int(time.time() * 1000),
        "mode": "paper" if os.getenv("PAPER_MODE","true").lower()=="true" else "live"
    }
