def run(context):
    return {
        "strategy": "volatility_06",
        "signal": "long" if context.get("bias", 0) >= 0 else "short",
        "confidence": 0.55,
        "stop_distance": 18,
        "target_profile": [1.2, 2.5, 3.8],
    }
