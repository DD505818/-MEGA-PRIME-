# ΩMEGA PRIME Δ v17.0.0 — Code Review Checklist

**Reviewer:** Claude Code (automated review)  
**Branch:** `claude/code-review-checklist-X7JWl`  
**Date:** 2026-05-03  
**Scope:** Full monorepo — all 28 services, critical path (risk → execution → audit)

---

## Summary

The architecture is sound and the risk-engine gate logic is well-structured. However, **six critical bugs** are present that would cause real-capital loss or authentication bypass in a live deployment. Several high-severity issues also undermine the stated safety invariants. These must be fixed before any live-capital approval process begins.

**Overall verdict:** Blocked for live capital — pass for continued paper trading.

---

## Checklist

### Functionality

- [x] Code does what it's supposed to do — *with exceptions noted below*
- [ ] Edge cases are handled — **FAIL: multiple edge-case gaps in risk and execution**
- [x] Error handling is appropriate — generally good across Go and Python services
- [ ] No obvious bugs or logic errors — **FAIL: six critical bugs identified**

#### CRITICAL — Logic Bugs

**C1 · `portfolio:open_positions` never decremented**  
`apps/execution-service/main.go:318`
```go
e.redis.IncrBy(ctx, "portfolio:open_positions", 1)
```
Every fill increments the open-positions counter. SELL fills and position closes never decrement it. Gate 8 will permanently tighten until it blocks all new trades. The counter must be decremented on SELL fills that reduce net position to zero.

**C2 · Gate 5 skips stale-data check when key is absent**  
`apps/risk-service/risk_engine.go:118–124`
```go
bookTSStr := r.redisString(ctx, bookTSKey)
if bookTSStr != "" {   // ← gate is bypassed when key missing
```
A missing `book_ts:<symbol>` key (broker disconnect, cold start, stale TTL) is silently treated as fresh data. The check must default to a rejection when the key is absent, not a pass.

**C3 · `seenSignalIDs` dedup map wipes entire state on overflow**  
`apps/risk-service/risk_engine.go:175–177`
```go
if len(r.seenSignalIDs) > 50_000 {
    r.seenSignalIDs = make(map[string]struct{})
}
```
Full reset creates a window where every previously-seen signal ID is temporarily re-valid. Under attack or surge conditions duplicate orders can slip through immediately after the reset. Replace with a time-windowed LRU or a ring-buffer of recent IDs.

**C4 · `MAKER.position` inventory never updated from real fills**  
`apps/agent-service/strategies/maker.py:14,28`
```python
self.position = 0.0
if ... abs(self.position) >= self.max_position:   # always 0.0
```
The in-agent inventory cap is permanently zero. The MAKER strategy will always emit signals regardless of accumulated inventory, bypassing the per-agent overexposure guard. Inventory must be injected from the portfolio service per tick.

**C5 · Static equity used for Kelly sizing in orchestrator**  
`apps/agent-service/orchestrator.py:17` / `apps/agent-service/normalizer.py:18`
```python
EQUITY = float(os.getenv("PORTFOLIO_EQUITY", "100000"))  # read once at startup
```
Position sizes are computed against the equity value set at startup. After losses, positions are oversized relative to actual equity; after gains, they are undersized. Equity must be read from Redis on every signal normalization.

**C6 · Daily PnL key never written by execution service**  
`apps/execution-service/main.go:306–318` — `updatePortfolio` writes position quantities and open-positions count but never writes `portfolio:daily_pnl`. Gate 6 (daily loss cap) always reads 0, making it permanently inoperative.

---

### Code Quality

- [x] Code is readable and well-structured
- [x] Functions are small and focused
- [x] Variable names are descriptive
- [ ] No code duplication — `_atr()` is copy-pasted identically into `box_theory.py`, `rev.py`, `maker.py`, `arb.py`, `gap.py`; belongs in a shared `utils.py`
- [x] Follows project conventions

#### Quality Notes

**Q1 · RSI approximation in REV strategy**  
`apps/agent-service/strategies/rev.py:41–47`  
Uses simple mean of gains/losses rather than Wilder's EMA (the standard). Values diverge from other systems' RSI readings, especially on short series. Replace with EWM: `ewm(com=period-1, adjust=False)`.

**Q2 · BoxTheory confidence is constant**  
`apps/agent-service/strategies/box_theory.py:60`  
Confidence is hardcoded to `0.7` for every signal. It conveys no discriminatory information and wastes the signal-quality layer. Wire it to ATR magnitude, rejection candle body/shadow ratio, or sweep depth.

**Q3 · `inspect.signature` called on every message in orchestrator**  
`apps/agent-service/orchestrator.py:79`  
`inspect.signature(strat.generate_signal)` is called inside the hot loop for every Kafka message and every agent. Cache the result per strategy at startup.

**Q4 · ATR duplication across five strategy files**  
`box_theory.py`, `rev.py`, `maker.py`, `arb.py`, `gap.py` each define an identical `_atr()` static method. Extract to `apps/agent-service/strategies/utils.py`.

**Q5 · Truth-Core `verifyHandler` is an unbounded full-table scan**  
`apps/truth-core/main.go:195–207`  
GET `/verify` reads every audit row in sequence. At production write rates this will be millions of rows and cause multi-second stalls. Add a `?since_id=` parameter and verify only incremental ranges between scheduled checkpoints.

---

### Security

- [ ] No obvious security vulnerabilities — **FAIL: critical auth bypass**
- [ ] Input validation is present — *partial*
- [ ] Sensitive data is handled properly — **FAIL: hardcoded credentials**
- [ ] No hardcoded secrets — **FAIL**

#### CRITICAL — Security

**S1 · Authentication bypass in NextAuth — any username grants operator access**  
`apps/web-ui/src/app/api/auth/[...nextauth]/route.ts:10–13`
```typescript
async authorize(credentials) {
  if (credentials?.username) {          // ← password is never checked
    return { id: credentials.username, name: credentials.username, role: 'operator' };
  }
  return null;
}
```
Any non-empty username authenticates as `role: 'operator'`. The kill switch, risk parameters, and live-trading controls are exposed to any unauthenticated user who knows the login URL. Implement real credential verification (bcrypt hash comparison or provider OAuth).

**S2 · `dev-token` hardcoded bypass in WebSocket gateway**  
`apps/websocket-gateway/index.js:189–191`
```js
// In paper-mode/dev allow a special dev token
return token === 'dev-token';
```
This bypass is active in all modes (the comment says "paper-mode/dev" but `verifyToken` is called unconditionally). Any client passing `?token=dev-token` receives full real-time access to all trading channels including `signals.approved`, `orders.fills`, and `emergency.halt`. Remove the bypass entirely; use a proper dev JWT in `.env.development`.

**S3 · Hardcoded PostgreSQL credentials in truth-core**  
`apps/truth-core/main.go:231`
```go
dsn = "postgresql://postgres:omega@postgres:5432/omega"
```
The default DSN contains a plaintext password committed in source. If `POSTGRES_DSN` is not set, the application silently uses this credential. Any process with access to the running container or source repository gains unrestricted database access. Remove the default; make the app fatal on missing `POSTGRES_DSN`.

**S4 · Truth-Core `/append` and `/verify` endpoints have no authentication**  
`apps/truth-core/main.go:237–241`  
Any process that can reach port 8084 can write arbitrary audit events (undermining tamper-evidence) or trigger full-chain verification (DoS via table scan). Add service-to-service authentication (mTLS or a shared HMAC token) before these endpoints.

**S5 · Execution-service `/orders` endpoint has no authentication**  
`apps/execution-service/main.go:421–424`  
GET `/orders` returns all live order state without authentication. This leaks strategy IDs, sizes, venues, and fill prices to any network peer.

**S6 · Paper-signal silently promoted to live in signal validator**  
`apps/agent-service/signal_validator.py:37–40`
```python
if signal_mode == "paper" and LIVE_MODE:
    signal["mode"] = "live"  # promote to live in live env
```
A paper signal received in a live environment is silently mutated to live and allowed to proceed to execution. The intent appears to be handling misconfigured agents, but the correct behavior is to reject the signal so the agent's mode mismatch is surfaced as an error. Silent promotion masks misconfiguration and risks unintended live execution.

**S7 · JWT_SECRET defaults to `'change-me'`**  
`apps/websocket-gateway/index.js:23`  
The fallback value is public knowledge; any JWTs issued against it are forgeable. Require the env var; crash at startup if absent.

**S8 · Kafka communication is plaintext**  
`docker-compose.yml` — `KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092`  
All trading signals, order events, and emergency-halt messages travel unencrypted within the Docker network. In a cloud multi-tenant environment this is exploitable. Enable TLS listeners for production deployments.

**S9 · `PAPER_MODE` read at module import time in Python validator**  
`apps/agent-service/signal_validator.py:18`
```python
LIVE_MODE = os.getenv("PAPER_MODE", "true").lower() != "true"
```
This value is fixed for the lifetime of the process. Any runtime reconfiguration or hot-reload without a full restart will leave an incorrect mode. The Go risk engine re-reads `PAPER_MODE` per-request (line 101), creating a mismatch. Read the env var per call or use a shared Redis key.

---

## Risk & Execution — Special Attention

Per the stated invariant: *"If any part is weak (especially risk or execution), the whole system fails."*

| Gate | Implementation | Finding |
|------|---------------|---------|
| Gate 1 Kill Switch | `atomic.Bool` + Redis write | ✅ Correct |
| Gate 2 Circuit Breaker | `atomic.Bool` | ✅ Correct |
| Gate 3 Mode Check | Re-reads env per call | ✅ Correct |
| Gate 4 Broker Health | Redis `broker:status` check | ✅ Correct |
| Gate 5 Stale Data | **Missing key skips check** | ❌ **C2** |
| Gate 6 Daily Loss | Reads `portfolio:daily_pnl` | ❌ **C6** (key never written) |
| Gate 7 Max Drawdown | Peak equity comparison | ✅ Correct |
| Gate 8 Open Positions | Reads Redis counter | ❌ **C1** (counter drifts) |
| Gate 9 Asset Exposure | Redis per-symbol key | ✅ Correct (if written) |
| Gate 10 Correlation | Redis per-pair key | ✅ Correct (if written) |
| Gate 11 Dedup | In-memory map | ❌ **C3** (wipe-on-overflow) |
| Gate 12 Spread Guard | Redis `book_spread` | ✅ Correct |
| Gate 13 Confidence | `< minConfidence` check | ✅ Correct |
| Gate 14 Risk/Trade | Kelly + notional + leverage | ✅ Correct |

**Gates 5, 6, 8, and 11 are effectively inoperative or unreliable as implemented.**

Kill cascade implementation is correct: `CompareAndSwap` prevents double-trigger, Redis write is synchronous, Kafka publish is best-effort with 5s confirmation window.

---

## Issue Priority

| ID | Severity | File | Description |
|----|----------|------|-------------|
| S1 | **Critical** | `web-ui/src/app/api/auth/[...nextauth]/route.ts:11` | Auth bypass — any username grants operator access |
| S2 | **Critical** | `websocket-gateway/index.js:190` | `dev-token` bypass active in all modes |
| C1 | **Critical** | `execution-service/main.go:318` | Open-positions counter never decremented |
| C6 | **Critical** | `execution-service/main.go:306–318` | Daily PnL key never written → Gate 6 inoperative |
| C2 | **High** | `risk-service/risk_engine.go:118` | Gate 5 silently passes on missing book-ts key |
| C3 | **High** | `risk-service/risk_engine.go:175` | Dedup map full-wipe creates duplicate-order window |
| S3 | **High** | `truth-core/main.go:231` | Hardcoded DB credentials in source |
| S6 | **High** | `signal_validator.py:40` | Paper signal silently promoted to live |
| C4 | **Medium** | `strategies/maker.py:14` | MAKER inventory never updated from fills |
| C5 | **Medium** | `orchestrator.py:17` | Static startup equity used for Kelly sizing |
| S4 | **Medium** | `truth-core/main.go:237` | Audit append/verify endpoints unauthenticated |
| S5 | **Medium** | `execution-service/main.go:421` | Orders endpoint unauthenticated |
| S7 | **Medium** | `websocket-gateway/index.js:23` | JWT_SECRET defaults to `'change-me'` |
| S8 | **Medium** | `docker-compose.yml` | Kafka plaintext in production config |
| Q1 | **Low** | `strategies/rev.py:41` | Non-standard RSI (simple mean vs Wilder EWM) |
| Q2 | **Low** | `strategies/box_theory.py:60` | Constant confidence value (always 0.7) |
| Q3 | **Low** | `orchestrator.py:79` | `inspect.signature` called in hot loop |
| Q4 | **Low** | Multiple strategy files | `_atr()` duplicated across 5 files |
| Q5 | **Low** | `truth-core/main.go:195` | Full-table scan on `/verify` |
| S9 | **Low** | `signal_validator.py:18` | `PAPER_MODE` frozen at import time |

---

## What Is Working Well

- **Risk gate structure:** All 14 gates are synchronous, ordered cheapest-to-most-expensive, and correctly block the execution path. The pattern is sound — the bugs are in the data the gates read, not the gate logic itself.
- **Kill cascade:** `CompareAndSwap` + Redis + Kafka + 5s confirmation timeout is correctly implemented.
- **Truth-Core audit chain:** SHA-256 hash chaining under `SERIALIZABLE` transaction isolation with row-level locking is correct and tamper-evident.
- **Signal validator:** Five-layer validation with schema, price, quantity, confidence, and stop/target direction checks is thorough.
- **Normalizer:** Kelly-fractional sizing with notional capping is correctly implemented.
- **TWAP/Iceberg slicing:** Randomized delays (10–50ms) prevent predictable order patterns.
- **Docker Compose healthchecks:** All infrastructure dependencies have proper health conditions before dependent services start.
- **Test coverage on critical path:** `test_risk_limits.py` covers halt logic including edge cases (None state, non-mapping, non-numeric); agent smoke tests verify signal contract.

---

## Recommended Fix Order

1. **Immediately (before any live-capital discussion):**
   - Fix S1 (auth bypass — check password)
   - Fix S2 (remove `dev-token`)
   - Fix C6 (write `portfolio:daily_pnl` in execution service)
   - Fix C1 (decrement open-positions on close)

2. **Before paper-to-live transition approval:**
   - Fix C2 (treat missing book-ts as stale)
   - Fix C3 (replace dedup map wipe with time-windowed LRU)
   - Fix S3 (remove hardcoded DSN fallback)
   - Fix S6 (reject paper signal in live env instead of promoting)
   - Fix C4 (inject live inventory into MAKER)
   - Fix C5 (read equity from Redis per tick)

3. **Before production hardening sign-off:**
   - Fix S4, S5 (add service auth to truth-core and execution HTTP endpoints)
   - Fix S7 (crash on missing JWT_SECRET)
   - Fix S8 (enable Kafka TLS)
   - Fix Q1–Q5 (quality improvements)
