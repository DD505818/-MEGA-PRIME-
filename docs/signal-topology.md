# Signal & Market Topology — Annotated Trace (Phase 1B.4)

Traced 2026-09-29 from producer to terminal consumer. Every edge below was
verified by reading the subscribing/publishing code, not from diagrams.

## The corrected signal path

```
agent-service (orchestrator.py)
  │  publishes agent signals
  ▼
signals.raw ──────────────────────────────────────────────────────┐
  │                                                               │ (observe only)
  ├─► fusion-engine ──► signals.fused ──► capital-allocator ──► signals.sized ──► risk-service ──► signals.approved ──► execution-service
  │        │                    │                  │                     │                    │                          │
  │   5s window, ≥2 agreeing    │           Kelly sizing,         14 gates incl.        risk_approved=true,      fills orders,
  │   agents/symbol+side,       │           overwrites            Gate 11 dedup         quantity = sized qty     pre-submit kill
  │   confidence-weighted       │           quantity              (durable SET NX)                               check, ledger +
  │   fusion, mints new         │                                                                  │             open_symbols set
  │   signal_id                 │                                                                  │
  │                             │                                                                  ├─► websocket-gateway (observe)
  │                             │                                                                  │
  │                             │                                                           signals.rejected ──► websocket-gateway (observe)
  │                             │
  │                      (observe: websocket-gateway)
  │
  └─► websocket-gateway (observe only)
```

### Why this ordering — the allocation race, proved absent

**The old wiring (before 1B.4):** risk-service consumed `signals.raw`
directly; capital-allocator consumed `signals.approved` and published
`signals.sized`; **nothing consumed `signals.sized`**; execution-service
consumed `signals.approved`. Two defects:

1. The allocator's Kelly sizing was silently discarded — execution traded
   the pre-allocation quantity.
2. Had execution consumed `signals.sized`, it would have traded a quantity
   risk never validated: Gate 14 (max notional, max leverage,
   risk-per-trade) ran on quantity Q₁, the allocator overwrote it with Q₂,
   execution traded Q₂. That is the allocation race — a TOCTOU between
   risk approval and size mutation.

**The corrected wiring (1B.4):** the allocator runs *before* risk
(`signals.fused → capital-allocator → signals.sized → risk-service`).
The quantity execution-service trades is therefore *exactly* the quantity
Gate 14 approved — the race is eliminated structurally, not by convention:

- risk-service subscribes to `signals.sized` (changed from `signals.raw`).
- capital-allocator subscribes to `signals.fused` (changed from
  `signals.approved`).
- execution-service still consumes only `signals.approved`.
- There is no path by which an un-fused, un-sized, or un-approved quantity
  reaches execution: fusion is consensus-gated (≥2 agents), allocation is
  Kelly-bounded, risk is the final choke point.

**Fail-closed consequence:** if fusion or the allocator is down, no
`signals.sized` messages exist and risk validates nothing — the system
halts rather than trading on raw agent output. This is intended.

**Dedup note:** fusion mints a new `signal_id` per fused signal; the
allocator preserves it. Gate 11's durable dedup therefore keys on the
fused ID — each consensus event is unique, redeliveries are rejected.

## The market path

```
market-data-service (Rust) ──► market.raw ──► feature-engine ──► features.norm ──► agent-service (orchestrator)
        │  (1B.4: REAL Kraken XBT/USD ticks;                          │  (windowed RSI/SMA/norm_price)
        │   was: synthetic Binance random-walk,                        ▼
        │   timestamp: 0)                                     Redis prices:<symbol>
        │
        └─► market.prices ──► portfolio-service (mark-to-market → portfolio:equity)
            (1B.4: NEW — this topic had NO producer; portfolio-service
             was subscribed to a dead topic and equity never marked)
```

- `market.prices` payload: `{"BTC/USD": <kraken last price>}` — the exact
  `map[string]interface{}` shape `portfolio-service.updateMarketPrices`
  consumes.
- On Kraken failure the service publishes nothing — fail-silent on data,
  never synthetic (the old `timestamp: 0` fabrication is deleted).

## Dead topics found and disposition

| Topic | Was | Now |
|-------|-----|-----|
| `signals.fused` | produced, **no consumer** (fusion bypassed) | consumed by capital-allocator ✅ |
| `signals.sized` | produced, **no consumer** (sizing discarded) | consumed by risk-service ✅ |
| `market.prices` | consumed by portfolio-service, **no producer** (equity never marked) | produced by market-data-service (real Kraken) ✅ |
| `portfolio.state` | bridged by gateway, **no producer** | still dead — flagged, not fabricated |

## Open topology items (NOT silently bridged)

1. **Symbol normalization.** Market data is now keyed `BTC/USD` (Kraken
   canonical, per the PAPER proof target). Agent signals in the repo use
   `BTCUSDT`. `portfolio-service.updateMarketPrices` matches on exact
   symbol keys, so mark-to-market will not apply until one canonical
   symbol is adopted end-to-end. Deliberately left as a follow-up — no
   silent aliasing.
2. **`portfolio.state`** has no producer. The gateway bridges it; nothing
   publishes it. Left dead and labeled rather than invented.
3. **Fusion consensus threshold** (≥2 agreeing agents, confidence ≥0.65)
   means the corrected chain is quiet until agents actually agree. In
   paper mode with few live agents this yields few or no fused signals —
   that is the honest behavior, not a bug to work around.
4. **`book_ts:<symbol>`** (1B.1 freshness) still has no canonical writer;
   Gate 5 therefore rejects until the market-data layer writes it.

## Verification

- `go build ./...` clean for risk-service, capital-allocator,
  execution-service, portfolio-service.
- `go test -race ./...` green for risk-service (includes the 1B.1–1B.3
  blocks); execution-service unit tests green.
- websocket-gateway: `node --check index.js` clean; existing jest suite
  covers auth (16 tests, unaffected by topic-list change).
- market-data-service: `cargo build` clean, `cargo test` 2/2 green
  (Kraken response-shape parsing). Toolchain: cargo 1.98.1.
