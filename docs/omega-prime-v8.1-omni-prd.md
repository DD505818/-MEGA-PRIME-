# ΩMEGA PRIME Δ v8.1 "OMNI" — Product Requirements Document

**Version:** 1.0  
**Date:** 2026-05-18  
**Status:** Production Readiness Specification

## TL;DR
This PRD defines a production-grade architecture for ΩMEGA PRIME Δ v8.1 OMNI, centered on a post-CRRP learning component (SQRA v2.0), deterministic risk vetoes, immutable audit, and one-command deployment.

## 1) Scope
- Build an autonomous trading platform that can run in paper, shadow, or live mode.
- Preserve strict risk primacy: no component may bypass AEGIS risk vetoes.
- Add private online learning via SQRA from realized outcomes only.

## 2) Non-Goals
- Not a custody provider.
- Not an execution venue.
- Not a guaranteed-return product.

## 3) System Layers
1. **Ingestion:** CCXT, Yahoo Finance, CoinGecko, policy/event feeds, wallet telemetry.
2. **Intelligence:** Feature engine (312 features), CRRP, SQRA, capital dominance protocol.
3. **Risk/Execution:** AEGIS (14 gates), execution hub, venue routing.
4. **Persistence/Audit:** PostgreSQL + Timescale, Redis, immutable truth-core with hash-chain.
5. **Presentation:** Next.js command terminal + onboarding + kill switch.

## 4) SQRA v2.0 Requirements
- **Input:** 312-d vector + CRRP + risk metrics.
- **Output heads:**
  - probability in `[0, 1]`
  - direction in `[-1, 1]`
  - slippage penalty `>= 0`
- **Training data:** realized paper/live PnL only.
- **Loop:** async mini-batch updates with gradient clipping.
- **Persistence:** local durable checkpoint + Redis backup.
- **Safety:** SQRA cannot execute trades directly.

## 5) Risk Controls
AEGIS must hard-enforce:
1. schema validation
2. kill switch state
3. circuit breaker
4. mode consistency
5. position limits
6. asset exposure
7. leverage
8. daily loss
9. drawdown
10. toxicity filter
11. confidence floor
12. liquidity
13. correlation
14. slippage model

## 6) Deployment Contract
- Single command bootstrap (`make up` or equivalent) must stand up all required services.
- Fresh install onboarding target: <10 minutes to paper trading readiness.
- Default mode must be **paper trading**.

## 7) SLO / Performance Targets
- Feature computation p99: `< 2 ms`
- CRRP inference p99: `< 50 ms`
- SQRA inference p99: `< 5 ms`
- Risk gate p99: `< 50 µs`
- End-to-end signal→order p99: `< 10 ms`

## 8) Validation & Testing
- Deterministic replay tests for signal/risk pipelines.
- Fault-injection for Redis/Postgres/network partitions.
- Monte Carlo scenario tests for ruin probability and drawdown controls.
- Backtest/live shadow parity checks for data and execution assumptions.

## 9) Security Baseline
- At-rest and in-transit encryption.
- JWT auth with strict TTL and rotation.
- Signed decision artifacts and tamper-evident audit records.
- Hardware + software kill switch paths.

## 10) Acceptance Criteria (MVP Production)
- All 23 services start cleanly in compose environment.
- Paper-mode trading loop runs without manual intervention.
- AEGIS vetoes unsafe trades under synthetic adverse scenarios.
- SQRA checkpoints recover after process restart.
- UI exposes risk status, mode, kill-switch state, and audit trail health.

## 11) Delivery Checklist
- [ ] Service contracts and schema registry finalized.
- [ ] Risk policy defaults approved.
- [ ] Runbooks for incident response and rollback.
- [ ] Backup/restore drill passed.
- [ ] Compliance/legal review completed for target jurisdiction.

