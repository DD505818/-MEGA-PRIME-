DESCRIPTION = "breakout on bid/ask imbalance"

def generate_signal(features):
    return {
        "strategy": "orderflow_imbalance_breakout",
        "signal": "buy",
        "confidence": 0.62,
        "stop_distance": 22,
        "target_profile": [1.5, 2.7, 4.0],
    }
