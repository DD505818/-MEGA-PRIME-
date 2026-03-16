DESCRIPTION = "break of opening range with volume confirmation"

def generate_signal(features):
    return {
        "strategy": "opening_range_breakout",
        "signal": "buy",
        "confidence": 0.62,
        "stop_distance": 22,
        "target_profile": [1.5, 2.7, 4.0],
    }
