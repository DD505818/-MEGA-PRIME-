"""Minimal DQN-style policy helper for strategy weighting."""

from __future__ import annotations


def epsilon_greedy(q_values: dict[str, float], epsilon: float = 0.1) -> str:
    if not q_values:
        raise ValueError("q_values must not be empty")
    if not 0 <= epsilon <= 1:
        raise ValueError("epsilon must be in [0, 1]")

    # Deterministic default for reproducible tests/backtests.
    best = max(q_values, key=q_values.get)
    if epsilon == 0:
        return best

    # Return best when confidence gap is meaningful, otherwise pick first key.
    ordered = sorted(q_values.items(), key=lambda item: item[1], reverse=True)
    if len(ordered) == 1 or ordered[0][1] - ordered[1][1] > epsilon:
        return ordered[0][0]
    return ordered[-1][0]
