def run(context: dict) -> dict:
    return {
        "strategy": "liquidity sweep reversal",
        "signal": "buy",
        "confidence": 0.63,
        "stop_distance": 18.0,
        "target_profile": {"tp1": 1.0, "tp2": 2.2}
    }
