def classify_regime(volatility: float, trend: float):
    if volatility > 1.8:
        return "high_vol"
    if trend > 0.5:
        return "trend"
    return "mean_reverting"
