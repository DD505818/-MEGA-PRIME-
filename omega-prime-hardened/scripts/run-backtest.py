#!/usr/bin/env python3
"""Run a simple deterministic backtest from a CSV of close prices."""

from __future__ import annotations

import argparse
import csv
from pathlib import Path


def run(prices: list[float], starting_cash: float = 10000.0) -> dict[str, float]:
    if len(prices) < 2:
        raise ValueError("need at least two prices")

    shares = 0.0
    cash = starting_cash

    for i in range(1, len(prices)):
        prev_price = prices[i - 1]
        price = prices[i]
        if price > prev_price and cash >= price:
            shares += 1
            cash -= price
        elif price < prev_price and shares >= 1:
            shares -= 1
            cash += price

    equity = cash + shares * prices[-1]
    return {"ending_cash": cash, "ending_shares": shares, "ending_equity": equity}


def load_prices(path: Path) -> list[float]:
    values: list[float] = []
    with path.open(newline="", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        for row in reader:
            values.append(float(row["close"]))
    return values


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--input", required=True, help="CSV with `close` column")
    args = parser.parse_args()

    prices = load_prices(Path(args.input))
    result = run(prices)
    print(result)


if __name__ == "__main__":
    main()
