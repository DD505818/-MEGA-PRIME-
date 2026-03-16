DESCRIPTION = "reversion at HTF liquidity zones"

def generate_signal(features):
    return {
        "strategy": "mean_reversion_liquidity_zones",
        "signal": "buy",
        "confidence": 0.62,
        "stop_distance": 22,
        "target_profile": [1.5, 2.7, 4.0],
    }
