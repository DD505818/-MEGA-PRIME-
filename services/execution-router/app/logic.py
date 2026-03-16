EXCHANGES = ["binance", "kraken", "coinbase", "bybit", "alpaca"]

def select_exchange(candidates):
    ranked = sorted(
        candidates,
        key=lambda x: (x["spread"], -x["liquidity"], x["latency_ms"]),
    )
    return ranked[0]
