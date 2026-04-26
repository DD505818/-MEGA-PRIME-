# ΩMEGA PRIME Δ — Optimization Roadmap Execution Manifest

Linked issue: #37

This document converts the optimization blueprint into a repo-level execution plan. It is intentionally risk-first: every profit-expansion feature must preserve hard safety invariants, deterministic auditability, and paper-first deployment.

## Non-Negotiable Safety Invariants

1. No order bypasses AEGIS Governor.
2. Kill state overrides every allocator, model, strategy, and execution route.
3. PAPER_MODE remains the default until broker adapters, execution semantics, and validation artifacts are independently verified.
4. Every signal must be normalized before validation.
5. Every rejected signal/order must include a machine-readable reason.
6. Every model decision must include model version, feature schema version, and timestamp.
7. No profitability claims are promoted without matching backtest, walk-forward, Monte Carlo, paper, and audit evidence.

## Execution Sequence

### Phase 1 — Signal Quality Gate

Goal: stop low-quality/counter-regime signals before they reach risk.

Deliverables:

- ALPHA regime label attached to each signal.
- BETA structural zones emitted by the feature engine.
- GAMMA validator rejects signals with explicit reasons.
- DELTA footprint/order-flow features attached where available.

Acceptance checks:

- Signals missing regime or structural context are rejected or routed to conservative paper-only mode.
- Unit tests cover trend, mean-reversion, neutral, missing-feature, and stale-feature scenarios.
- Strategy output keeps the canonical signal contract.

### Phase 2 — Risk Governor Expansion

Goal: allow intelligent sizing only inside hard risk limits.

Deliverables:

- Dynamic leverage module with EWMA volatility and drawdown scaling.
- Correlation gate inside AEGIS.
- Per-strategy circuit breakers.
- Session-aware exposure caps.

Acceptance checks:

- `equity <= 0` never produces positive order sizing.
- Volatility spike guard locks leverage to 1.0x.
- Single-position portfolios do not false-trigger correlation blocks.
- Daily loss, max drawdown, per-trade risk, and kill state always override leverage.

### Phase 3 — CAFÉ-RC Fusion

Goal: combine signals without correlation drag or overfitting incentives.

Deliverables:

- `cafe-rc` service consuming `signals.raw` and publishing `fusion.signals`.
- Rolling Information Coefficient per strategy.
- Top-3 weighted signal fusion.
- Recursive Kelly multiplier with hard cap.

Acceptance checks:

- Fewer than 3 signals does not crash service.
- Missing IC falls back to conservative equal/low weight.
- Fused signal remains subordinate to AEGIS risk approval.

### Phase 4 — Execution Edge

Goal: reduce slippage, recover partial fills, and route intelligently.

Deliverables:

- Square-root impact model.
- Adaptive limit repricing with bounded slippage.
- Venue scoring wired into actual route selection.
- `PARTIALLY_FILLED` FSM state and remaining-quantity recovery.

Acceptance checks:

- Partial fills persist and reconcile.
- Repricing cannot exceed configured slippage/risk limits.
- Kill event cancels or blocks open orders within the configured latency budget.

### Phase 5 — Operator Terminal

Goal: make risk state, live/paper state, and emergency controls obvious.

Deliverables:

- Trading Terminal with candlesticks, overlays, signal markers, and execution blotter.
- Risk & Controls page with START, STOP, PAUSE, and KILL.
- Paper/live mode warnings.
- Order entry risk preview before submit.

Acceptance checks:

- KILL/STOP are always visible or one tap away.
- Paper/live state is unmistakable.
- UI never implies live-readiness while broker adapters are unverified.

### Phase 6 — Validation Wall

Goal: prevent deployment based on attractive but unproven metrics.

Deliverables:

- Walk-forward suite with purged cross-validation and embargo.
- Deflated Sharpe and Probability of Backtest Overfitting reporting.
- Monte Carlo simulations with fat tails and regime switching.
- Chaos test for in-flight orders during risk-engine failure.

Acceptance checks:

- CI fails when hard risk invariants break.
- Validation artifacts are versioned.
- Backtest assumptions are explicit.
- Paper/live promotion requires evidence, not narrative.

## Implementation Guardrails

- Go: latency-critical services such as risk, execution, fusion, and portfolio reconciliation.
- Python: analytics, strategy research, model training, and backtest pipelines.
- Protobuf or strict JSON contracts for service boundaries.
- No duplicated risk logic outside AEGIS.
- No hardcoded secrets, fallback JWT secrets, or production API keys.

## Initial PR Slice

The first implementation PR should be small and verifiable:

1. Add dynamic leverage module with tests.
2. Add correlation-gate interface into AEGIS without changing order approval semantics yet.
3. Add explicit rejection reason schema.
4. Add CI tests proving kill/drawdown/daily-loss override leverage.

## Definition of Done

The roadmap is complete only when each phase has:

- passing unit tests,
- integration tests where applicable,
- config documentation,
- audit events,
- explicit failure behavior,
- validation artifacts for any performance claim.
