# ΩMEGA PRIME Δ — Validation Laboratory

The gauntlet every edge candidate must pass **before PAPER**.
Doctrine: `EDGE NOT PROVEN`, `LIVE LOCKED`. After-cost means net of fees,
spread/slippage, and funding. Failed candidates are killed, never tuned.

A `PASS` here qualifies a candidate for PAPER campaigns only. It is not
profitability, not risk approval, and never a path to LIVE (which requires
the separate independent certification program).

## Install / run

```bash
cd apps/validation-lab
pip install -e .            # numpy, pandas, scipy only
python -m pytest tests/ -q  # unit tests, synthetic data, fast
```

## Canonical layout

This package is the **single canonical validation lab**. The gauntlet
(`lab.py`) is what MIDAS consumes; the research tooling consolidated from
the former `backtests/lab/` lives alongside it so there is exactly one
implementation of each shared statistic. `backtests/lab/` now contains
only thin re-export shims — new code imports `validation_lab` directly.

| Module | Contents |
|---|---|
| `lab.py` | 7-stage fail-fast gauntlet + JSON report (the MIDAS contract) |
| `data.py` | Frozen venue CSV loading with SHA-256 manifest verification |
| `dataset.py` | Snapshot sealing: schema detection (bars/ticks), hard quality gate, canonical CSV + `manifest.json` |
| `edge_input.py` | Load only verified sealed snapshots for edge search |
| `splits.py` | Purged walk-forward fold boundaries (anchored/rolling, purge + embargo, timestamp metadata) |
| `walkforward.py` | Purged K-fold CV index splits + per-fold OOS scoring (gauntlet stage 3) |
| `metrics.py` | Sharpe, Sortino, max drawdown, profit factor, total/annualized return, causal strategy returns, per-candidate metric table |
| `overfit.py` | Deflated Sharpe (ratio + research probability), probabilistic Sharpe, CSCV/PBO, parameter sensitivity, minimum backtest length |
| `bootstrap.py` | Stationary + circular-block Sharpe CIs; empirical contiguous-block path bootstrap (terminal return / drawdown / ruin) |
| `montecarlo.py` | Parametric Monte Carlo: Student-t fat tails, 2-state regime switching |
| `nulls.py` | Sign-flip permutation, random-timing, buy-and-hold null tests |
| `costs.py` | After-cost accounting (fees/spread/funding) with 1x/2x/3x stress |
| `cli.py` | `omega-validation-lab` research CLI: `prepare` / `verify` / `folds` / `evaluate` |

Note the two split generators are different tools, not duplicates:
`splits.purged_walk_forward` produces time-ordered train/test fold
*boundaries* for edge search, while `walkforward.purged_cv_splits`
produces purged K-fold *index splits* used to score the gauntlet's
out-of-sample Sharpe.

## API

```python
from validation_lab import LabConfig, run_gauntlet, write_report

def sma_cross_factory(params):
    # must return a Series of target positions in {-1, 0, 1}, indexed like df,
    # computed CAUSALLY (bar t may only use data up to bar t)
    ...

config = LabConfig(fee_bps=10.0, spread_bps=5.0, seed=42)
report = run_gauntlet(
    "/path/to/coinbase_BTC-USD_1h.csv",
    sma_cross_factory,
    {"fast": 20, "slow": 50},
    config,
    candidate="sma-cross-20-50",
)
write_report(report, "report.json")
```

`signal_factory(params) -> Series`: positions in {-1, 0, 1}. Strategy returns
are `position[t-1] * asset_return[t]` — no lookahead by construction.
`base_params` holds the candidate's numeric parameters; the lab perturbs them
itself for the sensitivity stage. There are no tuning hooks: the config is
fixed at construction.

## Stage order and pass criteria

Every stage runs on **after-cost** returns. Stages run in order; the first
`FAIL` stops the run and later stages are marked `NOT RUN`.

| # | Stage | What it does | Pass criterion (defaults) |
|---|-------|--------------|---------------------------|
| 1 | `data_integrity` | SHA-256 of the CSV vs the venue `MANIFEST.json`; schema/OHLC/monotonicity checks | loads, hash matches |
| 2 | `costs` | turnover × (fee+spread), funding on \|position\|; stress at 1x/2x/3x | net Sharpe > `min_net_sharpe` (0.0) |
| 3 | `walkforward` | purged K-fold CV with embargo (Lopez de Prado): training labels overlapping a test fold are purged, an embargo zone after each fold is dropped | mean OOS fold Sharpe ≥ `min_oos_sharpe` (0.5) **and** ≥ `min_positive_fold_frac` (0.6) of folds positive |
| 4 | `nulls` | sign-flip permutation p-value; random-timing (circular position shift) p-value; buy-and-hold Sharpe-gap test | both p-values ≤ `max_pvalue` (0.05) |
| 5 | `bootstrap` | stationary bootstrap (Politis–Romano, geometric blocks) CI for Sharpe | 95% CI lower bound > 0 |
| 6 | `montecarlo` | Student-t (MLE fit) and 2-state regime-switching path simulations | P(total return < 0) ≤ `max_mc_loss_prob` (0.5) under regime-switching |
| 7 | `overfit` | ±20% parameter perturbation (must survive); deflated Sharpe ratio vs all tried configs; track-record ≥ minimum backtest length | DSR ≥ `min_dsr` (0.95), sensitivity survives, length adequate |

### Methodological notes

- **After-cost first.** Costs are applied before any statistic is computed.
  A gross edge that dies at 1x costs never reaches stage 3.
- **Purge + embargo.** Financial folds leak: a label formed at the end of
  training overlaps the start of testing. Stage 3 removes that overlap and an
  embargo zone, so OOS means OOS.
- **Sign-flip, not shuffle.** Naively shuffling a return series preserves its
  multiset, hence its Sharpe, exactly — the test would be vacuous (p ≡ 1).
  The lab flips return *signs* (Rademacher permutation), the exact null for a
  Sharpe statistic under H0.
- **Deflated Sharpe.** Selects against multiple testing: the more
  configurations tried, the higher the bar (`trial_sharpes` come from the
  sensitivity perturbations plus the base config).
- **Deterministic.** Every stochastic stage draws from `np.random.default_rng`
  seeded from `config.seed` (+ stage offset). Same inputs → same report,
  bit for bit. No network calls anywhere.

## Report format

`run_gauntlet` returns a JSON-serializable dict:

```json
{
  "candidate": "sma-cross-20-50",
  "verdict": "FAIL",
  "failed_stage": "costs",
  "config": { "fee_bps": 10.0, "spread_bps": 5.0, "...": "..." },
  "data": { "file": "coinbase_BTC-USD_1h.csv",
            "sha256": "4aa57aa2...", "rows": 26270,
            "first": "2023-10-01T00:00:00Z", "last": "2026-09-29T23:00:00Z",
            "venue": "coinbase", "pair": "BTC-USD", "repo_commit": "da33d41d" },
  "stages": {
    "data_integrity": { "status": "PASS", "metrics": {...}, "threshold": null },
    "costs":          { "status": "FAIL", "metrics": {"net_sharpe": -0.42, ...},
                        "threshold": {"min_net_sharpe": 0.0} },
    "walkforward":    { "status": "NOT RUN", "metrics": {}, "threshold": null }
  },
  "doctrine": "EDGE NOT PROVEN unless verdict == PASS on all stages; ..."
}
```

Every number is traceable: `config` records all thresholds and the seed,
`data` records the exact frozen bytes hashed, and each stage records its
metrics beside the threshold it was judged against.

## Demo

`demo_sma_cross.py` runs a trivial SMA-cross (20/50) candidate against the
frozen Coinbase 1h dataset and writes `demo_report.json`. It is expected to
FAIL — the point is the harness, not the signal.

```bash
python demo_sma_cross.py
```
