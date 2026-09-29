# Phase 1B Review — Evidence per Sub-phase

Each block: the exact reproduced failure, the fix, the proving test(s),
and the emitted fail-closed audit entry. Code is uncommitted in the working
tree; each sub-phase is kept separable for its own PR (file lists below).

## 1B.1 — Fail-closed freshness (Gate 5)

**Failure reproduced.** `book_ts:<symbol>` was never written by any
producer (market-data-service emitted synthetic ticks with `timestamp: 0`
and never touched Redis). The old Gate 5 skipped the staleness check when
the key was absent — a silent feed was approved as if fresh.

**Fix** (`apps/risk-service/risk_engine.go`). Gate 5 now requires a
present, parseable, positive, non-future, fresh-enough `book_ts:<symbol>`
(Unix ms). Reject reasons: `GATE5_NO_BOOK_TS`, `GATE5_BAD_BOOK_TS`,
`GATE5_ZERO_BOOK_TS`, `GATE5_FUTURE_BOOK_TS_<n>ms`, `GATE5_STALE_BOOK_<n>ms`.
Per-symbol override via `STALE_BOOK_SECONDS_<SYMBOL>`. Rejection only —
never flatten, never approve.

**Proving tests** (`freshness_test.go`, 8 tests, `go test -race` green):
missing key, zero timestamp (the synthetic-feed case), bad/zero/future/
stale timestamps, fresh-tick approval, per-symbol override, and
every-rejection-audited.

**Fail-closed audit entry.** Every rejection calls
`auditFreshnessFailClosed` → `recordControlAudit("risk.gate5",
"risk-engine", true, "fail_closed", "symbol=… reason=…")` → stdout
`CONTROL_AUDIT` + `control:audit` Redis stream + TruthCore append, or a
`control:audit:gaps` gap record if TruthCore is down. An earlier
per-symbol 60s cooldown was **removed**: the standing rule is every
fail-closed event audited, no exceptions, no aggregation.

**Files for PR 1B.1:** `risk_engine.go` (Gate 5 + audit helper),
`freshness_test.go`, `fake_redis_test.go` (shared fake — also used by 1B.2/1B.3).

**Undone:** no canonical writer of `book_ts:<symbol>` exists yet, so Gate 5
rejects all signals until one is wired. Documented, not faked.

## 1B.2 — Position-count reconciliation

**Failure reproduced.** `portfolio:open_positions` was incremented on every
fill by execution-service and never decremented (BUY then SELL-to-close →
count 2, actual open 0), while portfolio-service overwrote the same key
with its own `len(Positions)` — two writers, two semantics, guaranteed
drift. Gate 8 decided on this number.

**Fix.**
- `apps/execution-service/main.go` `updatePortfolio`: fills ledger
  `portfolio:fills:<symbol>` (LPUSH, written **before** the position key),
  `portfolio:tracked_symbols` set, then idempotent `portfolio:open_symbols`
  set maintenance (SADD when |qty|>eps, SREM when dust/zero — the close
  removes the symbol on the same fill event). The old `IncrBy` counter is
  deleted; with SADD/SREM there is no counter to drift.
- `apps/portfolio-service/main.go` `publishState`: no longer writes the
  contested key.
- `apps/risk-service/risk_engine.go` Gate 8: `SCARD portfolio:open_symbols`.
- `apps/risk-service/reconcile.go` (new): every 60s, recompute each
  symbol's expected quantity from the fills ledger, diff against
  `portfolio:position:<symbol>`, and verify set/quantity consistency.
  Any divergence → audit + `risk.alerts` page + `activateKillSwitch`.
- `apps/execution-service/main.go` `processSignal`: pre-submit kill check
  reads durable `kill_switch` before routing; on `1` the order goes
  CANCELLED (`KILL_SWITCH_ACTIVE_AT_SUBMIT`). This narrows the 1A.6 gap —
  `docs/kill-switch-status.md` is now **PARTIAL** (the check fails open on
  Redis errors — `.Val()` ignores them; failing closed on store error is
  open work).

**Proving tests** (`reconcile_test.go`, 6 tests; `portfolio_test.go` in
execution-service): healthy ledger/position/set agreement, injected
ledger-vs-position divergence → kill + durable flag + audit, set/quantity
inconsistency → kill, Gate 8 reads the set, Gate 8 ignores the legacy
counter key, `isOpenPosition` boundaries incl. dust and shorts.

**Fail-closed audit entry.**
`recordControlAudit("risk.gate8", "system:position-reconciler", true,
"divergence", "<symbol>: ledger=… position=…; …")` + `risk.alerts`
`POSITION_DIVERGENCE` page + kill reason `1B2_POSITION_DIVERGENCE`.

**Files for PR 1B.2:** `reconcile.go`, `reconcile_test.go`,
`risk_engine.go` (Gate 8), `store.go` (Set/List primitives),
`main.go` (loop wiring), execution-service `main.go` +
`portfolio_test.go`, portfolio-service `main.go`,
`docs/kill-switch-status.md`.

## 1B.3 — Durable dedup (Gate 11)

**Failure reproduced.** Dedup was an in-memory `seenSignalIDs` map: lost on
every restart, and wiped entirely every 50,000 entries (re-admitting old
IDs). Crash between submit and record → redelivered signal approved twice
→ double order.

**Fix** (`apps/risk-service/risk_engine.go` Gate 11). Atomic durable
check-and-set: `SET risk:seen_signal:<id> 1 NX EX <DEDUP_TTL_SECONDS>`
(default 24h) **inside validate, before** `signals.approved` is published.
No check-then-set race between replicas; record survives restarts. Redis
unavailable → `GATE11_DEDUP_STORE_UNAVAILABLE` (fail closed: cannot prove
uniqueness → reject; a dropped signal beats a duplicated position).

**Remaining loss window (not exactly-once).** The record is written
*before* `signals.approved` is published, so a crash after SET NX but
before Kafka delivery loses the signal — redelivery is then rejected as
a duplicate. This is duplicate prevention with an at-most-once loss
window, not exactly-once execution. The proving test below demonstrates
the record-before-publish ordering, not "crash between submit and
record".

**Proving tests** (`dedup_test.go`, 5 tests, `-race` green):
first-seen/duplicate, restart durability (new engine, same Redis),
crash-before-publish (record asserted present in Redis before any
publish; redelivery rejected — proves at-most-once, not double-submit),
store-down fail-closed, 32-goroutine concurrent duplicate → exactly 1
approval.

**Fail-closed audit entry.** Gate rejections flow to `signals.rejected`
with `reject_reason`; store-unavailable is a hard reject, not a bypass.

**Files for PR 1B.3:** `risk_engine.go` (Gate 11 + `dedupTTL`),
`store.go` (`SetNX`), `dedup_test.go`.

## 1B.4 — Signal topology, allocator ordering, real market.prices

**Failures mapped** (all verified in code, see `docs/signal-topology.md`):
- `signals.fused`: produced by fusion-engine, **zero consumers** — fusion
  bypassed entirely; risk validated raw agent signals.
- `signals.sized`: produced by capital-allocator, **zero consumers** —
  Kelly sizing silently discarded; execution traded pre-allocation size.
- **Allocation race:** Gate 14 validated quantity Q₁; the allocator
  overwrote it with Q₂ downstream of risk. Had execution consumed
  `signals.sized`, it would have traded a size risk never approved.
- `market.prices`: consumed by portfolio-service, **zero producers** —
  equity never marked to market.
- `market-data-service` fabricated data: synthetic Binance random-walk,
  `price = 63000 + counter`, `timestamp: 0`.

**Fix.**
- capital-allocator subscribes to `signals.fused` (was `signals.approved`).
- risk-service subscribes to `signals.sized` (was `signals.raw`).
- execution-service still consumes only `signals.approved` — the quantity
  it trades is now *exactly* the quantity Gate 14 approved. The race is
  eliminated structurally: no path exists for an un-fused, un-sized, or
  un-approved quantity to reach execution. Fail-closed consequence: if
  fusion or the allocator is down, risk validates nothing and the system
  halts rather than trading raw agent output.
- market-data-service rewritten: polls Kraken public Ticker (XBT/USD) and
  publishes real ticks to `market.raw` (real price/bid/ask/timestamp) and
  the `{symbol: price}` map to `market.prices`. On Kraken failure it
  publishes nothing — never synthetic.
- websocket-gateway observes `signals.fused`/`signals.sized` (read-only).

**Proof.** `docs/signal-topology.md`: annotated trace producer→consumer
for every topic, the old-vs-new wiring, and why the race is absent.
`go build` clean (risk, allocator, execution, portfolio);
`go test -race` green (risk-service full suite, execution-service);
gateway jest 16/16; `cargo build` + `cargo test` 2/2 green
(market-data-service, incl. Kraken response-shape tests).

**Files for PR 1B.4:** `docs/signal-topology.md`,
capital-allocator `main.go`, risk-service `risk_engine.go` (subscription),
websocket-gateway `index.js`, market-data-service `src/main.rs` +
`Cargo.toml`/`Cargo.lock`.

**Explicitly undone / not silently bridged:**
1. Symbol normalization: market data is keyed `BTC/USD` (Kraken
   canonical); agent signals use `BTCUSDT`. Mark-to-market key matching
   needs one canonical symbol end-to-end — follow-up, no silent aliasing.
2. `portfolio.state` still has no producer (gateway bridges a dead topic).
3. `book_ts:<symbol>` still has no canonical writer (Gate 5 rejects until
   wired).
4. Fusion consensus (≥2 agents, confidence ≥0.65) means the corrected
   chain is quiet until agents actually agree — honest, not a bug.
5. Phase 1A blockers from the 2026-09-29 review still stand before any
   merge: private-topology confirmation, SIGKILL test, WS handshake test,
   sync-audit redesign, doc corrections, legacy CI, integration tests,
   per-service secrets / rotation / rate-limiting / WS re-auth.
6. Known test flake (2026-09-29): `TestRestoreControlStateFailClosed`
   failed once in a combined `-race` run while the machine was also
   compiling rdkafka via cmake (heavy parallel load). The test waits on a
   50ms-interval background retry goroutine with a 3s deadline; under
   extreme CPU contention the goroutine can miss the window. 5/5 subsequent
   `-race` runs green. The production logic (assume-killed, unverified,
   retry-verifies, disarm/re-arm) is sound; the fragility is the test's
   timing assumption under load. Do not merge 1A with this unaddressed —
   either raise the deadline or gate the suite from running alongside
   heavy builds.
