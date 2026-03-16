DESCRIPTION = "reversal at swept liquidity pools"

def generate_signal(features):
    return {
        "strategy": "liquidity_sweep_reversal",
        "signal": "buy",
        "confidence": 0.62,
        "stop_distance": 22,
        "target_profile": [1.5, 2.7, 4.0],
    }
