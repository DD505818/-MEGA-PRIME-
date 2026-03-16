from dataclasses import dataclass
from typing import Dict

@dataclass
class VenueSnapshot:
    spread_bps: float
    liquidity_score: float
    latency_ms: float

WEIGHTS = {"spread": 0.5, "liquidity": 0.3, "latency": 0.2}


def score(snapshot: VenueSnapshot) -> float:
    return (
        (1 / (snapshot.spread_bps + 1e-9)) * WEIGHTS["spread"]
        + snapshot.liquidity_score * WEIGHTS["liquidity"]
        + (1 / (snapshot.latency_ms + 1e-9)) * WEIGHTS["latency"]
    )


def choose_exchange(venues: Dict[str, VenueSnapshot]) -> str:
    return max(venues.items(), key=lambda item: score(item[1]))[0]
