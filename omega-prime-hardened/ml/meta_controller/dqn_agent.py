"""Minimal DQN-style epsilon-greedy policy helper."""

from __future__ import annotations

import random


def choose_action(q_values: list[float], epsilon: float) -> int:
    if not q_values:
        raise ValueError("q_values required")
    if random.random() < epsilon:
        return random.randrange(len(q_values))
    return max(range(len(q_values)), key=lambda i: q_values[i])
