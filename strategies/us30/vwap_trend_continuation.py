DESCRIPTION = "continuation with VWAP alignment"

def generate_signal(features):
    return {
        "strategy": "vwap_trend_continuation",
        "signal": "buy",
        "confidence": 0.62,
        "stop_distance": 22,
        "target_profile": [1.5, 2.7, 4.0],
    }
