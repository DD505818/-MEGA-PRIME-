# TruthCore — Phase 4: independent hashes, immutable lineage, fill reconciliation, audit-or-no-trade

TruthCore is the append-only, SHA-256 hash-chained audit spine of ΩMEGA PRIME Δ.
Phase 4 closes the four gaps that kept it a passive log:

1. **Independent hashes** — verification no longer trusts the server's `/verify`.
2. **Immutable lineage** — the log is append-only at the SQL level, and every
   trade event links to its parent (approval → order → fill).
3. **Fill reconciliation** — independent cross-checks detect unapproved orders,
   double-spent approvals, and orphaned fills.
4. **Audit-or-no-trade** — no trade may exist without its audit record; a
   compromised or unreachable audit spine halts trading.

## 1. Independent hash verification

`apps/truthclient/` is the shared client. Its key property: it recomputes the
chain **locally** from raw entry bytes (`prev_hash`, `hash`, `event_type`,
canonical payload) fetched via `GET /entries` and `GET /head`. It never calls
the server's `/verify` — a compromised server cannot vouch for itself.

- `VerifyIndependent` — full from-genesis recomputation.
- `VerifyIncremental(tipID, tipHash)` — verifies only new entries, anchored
  against the previously verified tip. Detects:
  - tail truncation (head moved backwards),
  - front truncation (genesis anchor: oldest entry must be id=1 from `genesis`),
  - tip rewrite / fork (head hash changed without new entries),
  - id gaps and broken prev_hash links.
- Integrity failures return a typed `*IntegrityError`, so callers can
  distinguish tamper evidence from transport errors with `errors.As`
  (no substring matching).

The server exposes `GET /head` (chain tip) and `GET /entries?since_id=&limit=`
(full entries including `prev_hash`); `/recent` now also returns `prev_hash`.

## 2. Immutable lineage

- **SQL-level immutability**: a `BEFORE UPDATE OR DELETE` trigger on
  `audit_log` raises on any mutation — effective even for superuser
  connections. The only write path is the append-only `/append`.
- **Canonical trade-lifecycle event types** (constants shared in truthclient):
  - `aegis.approval_issued` — risk-service, after signing, before forwarding.
    Binds the full approval (approval_id, signal, bound fields, mode).
  - `vulture.order_submitted` — execution-service, after approval verification,
    before any routing. Binds order_id → approval_id and the bound fields.
  - `vulture.fill` — execution-service, after a fill. Binds order_id,
    approval_id, filled quantity, fill price.
  - `vulture.order_refused` — execution-service, best-effort, on any refusal.
- `GET /lineage?approval_id=` / `?order_id=` returns the linked events:
  approval → order(s) → fill(s), reconstructible from the log alone.

## 3. Fill reconciliation

`GET /reconcile` runs three rules over the trade-lifecycle events (pure
function `reconcileEntries`, unit-tested without a database):

| Rule | Meaning |
|---|---|
| `ORDER_WITHOUT_APPROVAL` | order references an unknown approval |
| `ORDER_APPROVAL_MISMATCH` | order fields diverge from its approval |
| `APPROVAL_DOUBLE_SPEND` | one approval backs >1 order (single-use violated) |
| `FILL_WITHOUT_ORDER` | fill references an unknown order |
| `FILL_APPROVAL_MISMATCH` | fill carries a different approval than its order |
| `FILL_QTY_MISMATCH` | fill quantity ≠ order quantity |

Returns `409` with `valid=false` on any violation, `200` with `valid=true`
when clean. An empty trade history reconciles as valid.

## 4. Audit-or-no-trade enforcement

**risk-service (AEGIS):** after signing an approval, the approval is appended
to TruthCore *before* the signal is forwarded to `signals.approved`. Append
failure → the signal is rejected with `TRUTHCORE_APPEND_FAILED` and a
critical alert. A nil/unconfigured client fails closed.

**execution-service (VULTURE):** after approval verification and the kill
check, `vulture.order_submitted` is appended *before* any routing. Append
failure → refusal with `AUDIT_UNAVAILABLE`. After a fill, `vulture.fill` is
appended; on failure the fill cannot be un-happened, so the order is marked
`audit_gap=true` and **all new submissions halt** (`AUDIT_HALTED`) until the
spine recovers. Refusals are best-effort audited (the refusal is already the
safe outcome).

**Tamper response:** risk-service re-verifies the chain independently every
60s (incremental; full from-genesis re-verification hourly to catch corruption
of already-verified history). A typed integrity failure engages the kill
switch (`TRUTHCORE_TAMPER_DETECTED` + critical alert). Mere unreachability
only alerts — the append gates already block new approvals while the spine is
down.

## 5. Write authentication

`TRUTHCORE_WRITE_SECRET`: when set on truth-core, `/append` requires
`Authorization: Bearer <secret>`. risk-service and execution-service send it
when configured. Compose fails fast if unset; without it the server logs a
warning and accepts unauthenticated writes (dev only). Read endpoints
(`/head`, `/entries`, `/recent`, `/reconcile`, `/lineage`, `/verify`) stay
open — they are observability, and the UI needs them.

## Threat model notes

- A compromised truth-core **server process** cannot forge a clean
  verification: verifiers recompute locally, and the hourly full pass
  re-reads history.
- A **database-level attacker** who rewrites history consistently changes the
  tip hash → caught by the incremental tip pin. Inconsistent corruption →
  caught by the hourly full pass. Row deletion is blocked by the trigger;
  front/tail truncation is caught by the genesis anchor and head comparison.
- The remaining trust root is the **shared write secret** (any holder can
  append — but cannot rewrite, and appends are attributable by event
  content) and the **verifier's initial tip** (first full verification must
  happen before compromise; in practice at deployment).

## Deferred (not in Phase 4)

- Key rotation for `TRUTHCORE_WRITE_SECRET` (restart-based rotation works today).
- Reconciler-driven auto-kill on `APPROVAL_DOUBLE_SPEND` (currently surfaced
  via `/reconcile`; the single-use Redis claim remains the runtime guard).
- Cross-venue fill reconciliation (Kraken execution reports) — PAPER only.
