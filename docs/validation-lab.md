# Validation Lab — Edge Search Data Contract

> Canonical implementation: the `validation_lab` package in
> `apps/validation-lab/validation_lab/` (dataset sealing, splits, statistics,
> CSCV/PBO, bootstraps, Monte Carlo, gauntlet). `backtests/lab/` is a
> compatibility shim only since the lab reconciliation
> (`phase/7-lab-reconciliation`).

## Purpose

Edge search must optimize against a stable historical artifact, not an
unversioned download, mutable API response, or live stream. The validation lab
provides that boundary.

The canonical flow is raw export → hard schema/integrity checks → sealed
snapshot → hash verification → purged walk-forward folds → edge search →
candidate return matrix → CSCV/PBO + deflated Sharpe + empirical block
bootstrap → versioned research evidence.

## Clean-data gate

A snapshot is marked `edge_search_ready=true` only after all hard checks pass.
The gate accepts either OHLCV bars or the canonical `market.raw` tick shape
(`exchange,symbol,price,bid,ask,volume,timestamp`). It rejects invalid
timestamps, NaN/Inf values, non-positive prices, negative volume, OHLC
envelope violations, crossed spreads, duplicate keys, and unexpected source
ordering. Canonical bytes are SHA-256 sealed.

Gap counts are reported, not silently repaired. Market closures and venue
outages need domain review; the lab does not invent bars.

## Leakage control

Walk-forward folds are constructed from unique timestamps. The training set
ends before a configurable exclusion region of `purge_bars + embargo_bars`,
and the test set begins after that region. This prevents the same timestamp
from appearing in both train and test and creates an explicit buffer for
overlapping labels/features.

## Multiple-testing diagnostics

The evaluation layer reports annualized Sharpe and Sortino, maximum drawdown,
profit factor, hit rate, deflated-Sharpe probability, CSCV Probability of
Backtest Overfitting, and empirical contiguous-block bootstrap return/drawdown
distributions.

These are diagnostics, not a profitability guarantee. Thresholds for research
promotion should be defined separately from the measurement code so the lab
does not move goalposts to fit a candidate.

## Reproducibility

Every sealed snapshot has a deterministic `manifest_id`. Search outputs should
record that ID plus the fold file and strategy/code commit SHA. A result
without those three identifiers is not reproducible evidence.

## LIVE isolation

The lab is offline research infrastructure. It has no broker adapter,
credential loading, order-submission API, or mechanism to alter PAPER/LIVE
runtime state.
