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
| `montecarlo.py` | MC v2 two-path engine: hardened parametric MC (dense) + regime-aware block/event bootstrap (sparse); PASS/FAIL/INDETERMINATE |
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
| 2 | `costs` | turnover × (fee+spread), funding on \|position\|; stress at 1x/2x/3x | net Sharpe > `min_net_sharpe` (0.0), 2x net Sharpe > `cost_stress_2x_min_sharpe` (0.0), and 3x net Sharpe ≥ `cost_stress_3x_min_sharpe` (-0.5) |
| 3 | `walkforward` | purged K-fold CV with embargo (Lopez de Prado): training labels overlapping a test fold are purged, an embargo zone after each fold is dropped | mean OOS fold Sharpe ≥ `min_oos_sharpe` (0.5), ≥ `min_positive_fold_frac` (0.6) of folds positive, ≥ `min_oos_trades` (100) total OOS position changes, and ≥ `min_oos_trades_per_fold` (10) per fold |
| 4 | `nulls` | sign-flip permutation p-value; random-timing (circular position shift) p-value; buy-and-hold Sharpe-gap test | both p-values ≤ `max_pvalue` (0.05) |
| 5 | `bootstrap` | stationary bootstrap (Politis–Romano, geometric blocks) CI for Sharpe | 95% CI lower bound > 0 |
| 6 | `montecarlo` | MC v2 two-path engine (see below) | P(total return < 0) ≤ `max_mc_loss_prob` (0.20) on the path's decision generator; INDETERMINATE also fails the stage |
| 7 | `overfit` | ±20% parameter perturbation (must survive); deflated Sharpe ratio vs all tried configs; track-record ≥ minimum backtest length | DSR ≥ `min_dsr` (0.95), sensitivity survives, length adequate; textbook per-period DSR is reported for Gate-A review against `textbook_dsr_gate_a_floor` (0.90), not used as a lab verdict gate |

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
  sensitivity perturbations plus the base config). The reported
  `textbook_dsr` uses per-period units for Gate-A review; until the campaign
  log exists, both DSR figures are candidate-local and do not correct for
  campaign-wide search.
- **Deterministic.** Every stochastic stage draws from `np.random.default_rng`
  seeded from `config.seed` (+ stage offset). Same inputs → same report,
  bit for bit. No network calls anywhere.

### Monte Carlo v2 (validation protocol v2)

The v1 parametric Student-t estimators degenerated on zero-inflated
(sparse) return series — e.g. a strategy active 3 of 7 days produced
df ≈ 0.1 and scale ≈ 1e-14, a formally computed gate decision carrying
almost no statistical information. MC v2 (`montecarlo.MC_ENGINE_VERSION =
"2.0.0"`) is a two-path engine:

- **Dense path** — the v1 machinery (Student-t fat tails + 2-state
  regime-switching), hardened with predetermined fit diagnostics. A fit is
  accepted only if all parameters are finite, 2.1 ≤ df ≤ 1e12, and
  scale ≥ 1e-10. The decision generator stays `regime_switching`.
- **Sparse path** — a regime-aware block/event bootstrap. It models
  P(R_t ≠ 0) separately from R_t | R_t ≠ 0: inactive gaps are resampled
  from the empirical gap distribution; active blocks (maximal nonzero
  runs) are resampled with replacement from regime-conditioned pools,
  preserving durations, within-block serial dependence, signs, empirical
  loss tails, directional exposure, and cost realization; a simulated
  per-bar 2-state volatility-regime path preserves regime occupancy and
  transitions and conditions block selection. A second estimator (fixed
  circular block bootstrap) and a seed-split rerun guard against
  estimator disagreement and simulation non-convergence.

Path selection is by **predetermined diagnostics computed before any gate
decision** and never by result favorability: `zero_frac ≥ 0.20` selects
the sparse path (with ≥ 30 nonzero observations required). The gated
quantity and threshold are unchanged: P(simulated total return < 0) ≤
`max_mc_loss_prob` (0.20).

The MC verdict is **PASS / FAIL / INDETERMINATE**. INDETERMINATE triggers
include: insufficient observations or nonzero observations, too few
active blocks, unstable parameter estimation, seed-split disagreement
beyond MC noise, material disagreement between the two sparse
estimators, and any nonfinite summary. INDETERMINATE **cannot advance** —
the lab maps it to stage FAIL. NaNs can never reach a gate decision;
all fit diagnostics persist in the report's `diagnostics` block.

### Report lineage and protocol versioning

`validation_protocol_version` is `"v2"` for this engine (`"v1"` denotes the
campaign #1–2 legacy engine — never compare v1 and v2 MC numbers across
protocols). Every report carries a `lineage` block:

```json
"lineage": {
  "strategy_spec_hash": "…", "data_snapshot_hash": "…",
  "code_commit": "…", "validation_protocol_version": "v2",
  "rng_seed": 42, "cost_model_version": "1.0.0",
  "mc_engine_version": "2.0.0", "preregistration_id": "…",
  "hypothesis_number": 0
}
```

`LabConfig` accepts the caller-supplied fields (`strategy_spec_hash`,
`preregistration_id`, `hypothesis_number`, `rng_seed` — defaulting to
`seed`); the lab fills in the data hash, code commit (best-effort
`git rev-parse`, else `"UNKNOWN"`), and component versions. Stage
names/order and PASS/FAIL/NOT RUN stage semantics are unchanged, and all
new fields are additive, so downstream JSON consumers are unaffected.

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
