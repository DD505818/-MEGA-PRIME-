EXCHANGES = ["binance", "kraken", "coinbase", "bybit", "alpaca"]

def select_exchange(candidates):
    if not candidates:
        raise ValueError("candidates must contain at least one venue quote")

    required = ("spread", "liquidity", "latency_ms")
    normalized = []
    for candidate in candidates:
        if not all(metric in candidate for metric in required):
            raise ValueError(f"candidate is missing required metrics: {required}")
        normalized.append(candidate)

    ranked = sorted(
        normalized,
        key=lambda x: (x["spread"], -x["liquidity"], x["latency_ms"]),
    )
    return ranked[0]
