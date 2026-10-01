# ΩMEGA PRIME Δ — MIDAS

The promotion gate between **AGENTS** and **AEGIS** in the authority chain:

```
DATA → MODELS → AGENTS → MIDAS → AEGIS → VULTURE → TRUTHCORE → VALIDATION
```

No strategy signal may flow toward AEGIS (`signals.raw` → risk-service)
unless MIDAS has promoted it on the basis of a **validation-lab PASS verdict
on after-cost returns**. Doctrine: `EDGE NOT PROVEN` by default; the gate
fails closed. Failed candidates are killed, never tuned.

## State machine

```
CANDIDATE → VALIDATING → PROMOTED → DEMOTED ──(fresh PASS report)──→ PROMOTED
CANDIDATE → VALIDATING → REJECTED   (may be re-evaluated with any report)
```

- Unknown strategy → treated as **not promoted** (gate blocks).
- `DEMOTED` requires a **fresh** PASS report to return: the report SHA-256
  consumed by the demoted promotion is recorded and can never be reused.
- There is no manual promotion path and no force flag. There is a manual
  *demotion* path (safe direction).

## Promoting a strategy

```bash
cd apps/agent-service
python -m midas.cli promote --strategy BoxTheory --report /path/to/report.json --by devon
```

Promotion requires ALL of:

1. `report.verdict == "PASS"` and `failed_stage is None`
2. every gauntlet stage (`data_integrity … overfit`) present with
   `status == "PASS"` — explicitly including the after-cost `costs` stage
3. the report's own thresholds at least as strict as MIDAS's bar
   (a report generated with weaker thresholds is rejected)
4. `report.data.sha256` matching a hash in a frozen dataset manifest
   (`$MIDAS_DATASET_DIR/*/MANIFEST.json`)
5. for a `DEMOTED` strategy, a report hash never consumed before

Anything else → `REJECTED` with the reason recorded. Promotion metadata
(report SHA-256, data SHA-256, key metric snapshot, operator, timestamp) is
stored in SQLite at `$MIDAS_DB_PATH`.

## Demotion (PAPER outcomes)

```bash
python -m midas.cli record-fill --strategy BoxTheory --pnl -12.50 --notional 1000
python -m midas.cli demote --strategy BoxTheory --reason "operator review"
python -m midas.cli status --strategy BoxTheory   # read-only, the safe default
```

Every recorded fill re-evaluates the strategy once at least
`MIDAS_DEMOTE_MIN_FILLS` (default 30) fills exist. Breach of either bar
demotes immediately:

| env | default | meaning |
|---|---|---|
| `MIDAS_DEMOTE_MIN_SHARPE` | `0.0` | trailing per-trade Sharpe floor |
| `MIDAS_DEMOTE_MAX_DRAWDOWN` | `0.25` | max drawdown on cumulative PnL |
| `MIDAS_DEMOTE_MIN_FILLS` | `30` | fills before evaluation |
| `MIDAS_DEMOTE_TRADES_PER_YEAR` | `252.0` | Sharpe annualization |

## Thresholds (all env, defaults mirror the lab's `LabConfig`)

| env | default |
|---|---|
| `MIDAS_MIN_NET_SHARPE` | `0.0` |
| `MIDAS_MIN_OOS_SHARPE` | `0.5` |
| `MIDAS_MIN_POSITIVE_FOLD_FRAC` | `0.6` |
| `MIDAS_MAX_PVALUE` | `0.05` |
| `MIDAS_MIN_DSR` | `0.95` |
| `MIDAS_SENSITIVITY_MIN_FRAC` | `0.5` |
| `MIDAS_MAX_MC_LOSS_PROB` | `0.20` |
| `MIDAS_DB_PATH` | `~/.omega-prime/midas.db` |
| `MIDAS_DATASET_DIR` | `~/workspace/datasets/omega-prime` |

Changing the bar is config, not code. Raising it is the only safe direction.

## Enforcement point

`orchestrator.py` checks `midas.is_promoted(name)` in the publish loop
**before** normalize/validate: unpromoted strategies are skipped, with one
warning per strategy per process. The 14 validation gates are untouched —
MIDAS is an additional gate upstream of them.

## Tests

```bash
cd apps/agent-service
python -m pytest midas/tests/ -q
```

## Doctrine notes

- A lab PASS qualifies a candidate for PAPER campaigns only. MIDAS promotion
  is not risk approval and never a path to LIVE (separate certification).
- The registry is local SQLite, not consensus: it records *this*
  operator's promotion decisions. The report SHA-256 chain (lab report →
  registry → TruthCore audit trail) is what makes a promotion auditable.
- If the registry is unreachable or corrupt, the gate fails closed:
  nothing emits.
