def detect_regime(features):
    return "trend" if features.get("vol",0)<0.3 else "volatile"
