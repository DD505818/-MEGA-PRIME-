# ΩMEGA Validation Lab

The validation lab is the offline research boundary for edge search. Raw market
data is never consumed directly by search code. A dataset must first pass the
quality gate, be canonicalized, hashed, and sealed into a snapshot.

## Hard invariants

- supported canonical schemas:
  - bars: `timestamp,symbol,open,high,low,close,volume`
  - ticks: `timestamp,exchange,symbol,price,bid,ask,volume` (matches `market.raw`)
- UTC-normalizable timestamps
- finite numeric values only
- strictly positive OHLC or price/bid/ask, non-negative volume
- valid OHLC envelopes and non-crossed tick spreads (`ask >= bid`)
- no duplicate `(symbol,timestamp)` keys
- deterministic `timestamp,symbol` ordering
- canonical SHA-256 hash recorded in `manifest.json`
- any post-seal mutation makes verification fail
- walk-forward splits exclude configurable purge + embargo bars
- research statistics are evidence only; they do not unlock LIVE execution

## Install

```bash
python -m pip install -r backtests/requirements.txt
```

## 1. Seal clean data

```bash
python -m backtests.lab.cli prepare \
  --input /path/to/raw-bars.csv \
  --output backtests/artifacts/btc-1m-v1 \
  --source kraken-export \
  --min-rows 10000
```

This writes `market-data.csv` plus `manifest.json`. The manifest binds the source
hash, canonical hash, schema version, symbol universe, time range, gap summary,
and deterministic manifest ID.

Schema is auto-detected by default. Out-of-order source rows fail closed. `--allow-reorder` exists only for data
that has been reviewed and intentionally canonicalized.

## 2. Verify before every search run

```bash
python -m backtests.lab.cli verify \
  --snapshot backtests/artifacts/btc-1m-v1
```

Edge-search code should consume only a snapshot that passes this command. Do
not point search jobs at raw downloads.

## 3. Generate leakage-safe folds

```bash
python -m backtests.lab.cli folds \
  --snapshot backtests/artifacts/btc-1m-v1 \
  --output backtests/artifacts/btc-1m-v1/folds.json \
  --train-bars 30000 \
  --test-bars 5000 \
  --purge-bars 120 \
  --embargo-bars 120
```

Fold boundaries are based on unique timestamps, so all symbols at the same
timestamp stay on the same side of a split.

## 4. Validate candidate return streams

After edge search produces aligned simple-return columns, run:

```bash
python -m backtests.lab.cli evaluate \
  --returns candidate_returns.csv \
  --output validation-report.json \
  --periods-per-year 525600 \
  --cscv-slices 8 \
  --simulations 5000 \
  --block-size 60
```

The report contains per-candidate return metrics, a deflated-Sharpe
probability, CSCV Probability of Backtest Overfitting diagnostics, and a
seeded contiguous-block bootstrap that resamples the empirical return
distribution rather than assuming Gaussian tails.

## Research boundary

The lab intentionally has no broker credentials, order submission path, or
LIVE switch. Passing the lab means the data/evaluation artifact is
reproducible enough for research. It does not establish future profitability,
broker execution quality, or LIVE readiness.
