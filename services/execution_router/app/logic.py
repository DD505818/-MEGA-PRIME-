EXCHANGES = ["binance", "kraken", "coinbase", "bybit", "alpaca"]


def _validated_candidate(candidate, required):
    if not isinstance(candidate, dict):
        raise TypeError("candidate must be a mapping")
    if not all(metric in candidate for metric in required):
        raise ValueError(f"candidate is missing required metrics: {required}")

    normalized = dict(candidate)
    for metric in required:
        try:
            normalized[metric] = float(candidate[metric])
        except (TypeError, ValueError) as exc:
            raise ValueError(f"candidate[{metric!r}] must be numeric") from exc
    return normalized


def select_exchange(candidates):
    if not candidates:
        raise ValueError("candidates must contain at least one venue quote")

    required = ("spread", "liquidity", "latency_ms")
    normalized = [_validated_candidate(candidate, required) for candidate in candidates]

    ranked = sorted(
        normalized,
        key=lambda x: (x["spread"], -x["liquidity"], x["latency_ms"]),
    )
    return ranked[0]
