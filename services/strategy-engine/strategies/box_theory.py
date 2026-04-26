from __future__ import annotations

from typing import Iterable, Sequence


class BoxTheory:
    def _calc_atr(self, highs: Sequence[float], lows: Sequence[float], closes: Sequence[float], period: int = 14) -> float:
        if period <= 0:
            raise ValueError("period must be positive")
        if not highs or not lows or not closes:
            return 0.0

        n = min(len(highs), len(lows), len(closes))
        if n < 2:
            return 0.0

        true_ranges: list[float] = []
        for i in range(1, n):
            high = highs[i]
            low = lows[i]
            prev_close = closes[i - 1]
            tr = max(high - low, abs(high - prev_close), abs(low - prev_close))
            true_ranges.append(tr)

        if not true_ranges:
            return 0.0

        window = true_ranges[-period:]
        return sum(window) / len(window)
