# PRD: ΩMEGA PRIME Δ

**Document:** ΩMEGA PRIME Δ Autonomous Trading Platform  
**Version:** 14.2.0  
**Status:** Product requirements and planning artifact, not a production-readiness certification.

## 1. Product Overview

ΩMEGA PRIME Δ is an autonomous, multi-agent, multi-asset algorithmic trading platform designed for individual professional traders. The system ingests real-time market data from multiple exchanges, computes technical features through a dedicated feature engine, generates trading signals via specialized strategy agents, validates every order against risk gates, and executes orders with deterministic state transitions and audit trails.

The platform operates in three modes:

- **BACKTEST:** deterministic historical replay.
- **PAPER:** live or synthetic market data with simulated execution.
- **LIVE:** supervised live trading only after explicit configuration and verification.

Paper mode must remain enabled by default. No order may bypass the risk service. The kill switch must reject new orders and initiate cancellation of open orders. Any latency target, drawdown target, ruin probability, or return target is treated as a validation target until measured by reproducible tests and audited deployment evidence.

## 2. Goals

### 2.1 Business Goals

- Enable individual traders to evaluate institutional-style risk controls without managing complex infrastructure.
- Provide an autonomous paper-trading experience before live capital is ever used.
- Support multi-asset trading from a single unified platform.
- Maintain a strategy library that can evolve through controlled validation.

### 2.2 User Goals

- Deploy the platform in paper mode with a single command.
- Monitor portfolio performance, agent status, execution state, and risk metrics through a professional web terminal.
- Create custom strategies using natural language through an AI Strategy Studio.
- Halt trading with a deliberate hold/slide-to-confirm kill switch.
- Verify that every trade decision is logged to an auditable event trail.
- Trust that risk limits are enforced before any order reaches execution.

### 2.3 Non-Goals

- Social trading, public leaderboards, or consumer copy-trading.
- Mobile-first simplified trading flows as the primary product direction.
- Educational tutorials as a core product module.
- Unsafely bypassing broker verification or risk controls to accelerate live trading.
- Unsupported claims of live profitability, production readiness, or regulatory compliance.

## 3. User Personas

### Professional Trader

An experienced individual trader managing personal capital. Comfortable with technical indicators, multi-asset portfolios, and risk management. Needs automation, safety controls, and clear system health visibility.

### Quantitative Developer

A user extending the platform with custom strategies. Comfortable with Python and deterministic backtesting. Needs clean APIs, documented contracts, and reproducible validation.

### System Operator

A DevOps-minded user deploying to Kubernetes. Needs Helm charts, monitoring, runbooks, and clear failure modes.

## 4. Role-Based Access

- **Trader:** dashboard, terminal, agent controls, Strategy Studio, and kill switch. Can deploy validated strategies with approval. Cannot bypass hard risk gates.
- **System Operator:** infrastructure, deployment scripts, monitoring, and runbooks. Cannot bypass application risk gates.

## 5. Functional Requirements

### 5.1 Autonomous Strategy Execution — P0

- Strategy agents run concurrently and generate signals from market/feature data.
- Signals are normalized to a canonical contract before risk validation.
- Box Theory must never use look-ahead data; it may only use prior-day levels and intraday data available up to the current bar.
- Agents must safely return `None` when required data is unavailable.

### 5.2 Risk Gating — P0

- Every order must pass through AEGIS Governor before execution.
- Risk checks include daily loss, max drawdown, per-trade risk, max open positions, max asset exposure, and correlation clustering.
- Dynamic leverage can scale only within hard risk limits.
- Kill state overrides every strategy, allocator, and execution route.

### 5.3 Execution Determinism — P0

- Orders follow a finite state machine such as `NEW -> OPEN -> PARTIAL -> FILLED` or `CANCELLED`.
- TWAP, VWAP, and iceberg execution must be bounded by risk and slippage limits.
- Order IDs must be idempotent.
- Partial fills must persist and reconcile.

### 5.4 AI Strategy Studio — P1

- Accept natural-language strategy descriptions.
- Generate executable Python agent candidates in a sandbox.
- Run backtests with explicit validation metrics.
- Require operator approval before deployment.
- Failed strategies must display concrete rejection reasons.

### 5.5 Self-Healing Recovery — P1

- Health monitoring must distinguish degraded, failed, and unavailable states.
- Recovery sequence should be staged: recycle connections, restart pod, then scale up.
- Repeated failure escalates to human operator.
- All recovery attempts must be audited.

### 5.6 Compliance and Auditability — P1

- Audit events must be append-only and tamper-evident.
- Secrets must not be committed to the repo.
- API keys must be loaded through secure runtime configuration.
- Compliance features must be represented as implementation targets unless legally reviewed and validated.

## 6. User Experience

### 6.1 First-Time Flow

- User clones the repository.
- User runs `cp .env.example .env && make up`.
- System starts in paper mode by default.
- User opens the local dashboard.
- Kubernetes users deploy via Helm once manifests are verified against implemented services.

### 6.2 Core Screens

#### Dashboard

Shows portfolio value, daily PnL, drawdown, allocation, agent status, service status, and emergency controls. Critical controls must remain visible under stress.

#### Terminal

TradingView-style workspace with candlestick chart, timeframe selection, symbol search, overlays, order entry, risk preview, and execution blotter.

#### Risk & Controls

Dedicated START, PAUSE, STOP, and KILL controls. Risk parameters are displayed and editable only within safe bounds.

#### Strategy Studio

Natural-language strategy input, generated code review, sandboxed backtest results, validation metrics, and approval workflow.

#### Audit

Append-only event timeline with filters for risk, execution, signal, kill, auth, and system events.

## 7. UX Edge Cases

- Paper/live mode must be unmistakable.
- Empty agent states must direct users toward Strategy Studio or configuration.
- Missing market data must show waiting/disconnected state.
- Widget-level error boundaries must keep the rest of the dashboard functional.
- WebSocket disconnects must be clearly visible.

## 8. Success Metrics

### User-Centric Metrics

- Time from clone to first paper signal: under 5 minutes.
- Strategy prompt to backtest result: under 3 minutes.
- Dashboard LCP: under 2.5 seconds.
- Critical controls accessible on desktop and mobile.

### Technical Metrics

- Tick-to-Kafka latency p99 target: under 5ms.
- Signal-to-order ACK p99 target: under 100ms.
- Recovery time target: under 60 seconds.
- Backtest PBO target: below 0.05.
- Deployed strategy deflated Sharpe target: above 1.5.

All metrics above are targets until backed by reproducible measurements.

## 9. Technical Considerations

### Integrations

- Exchanges and brokers: Binance, Coinbase, Kraken, Alpaca, Interactive Brokers, as verified adapters become available.
- Kafka for service event backbone.
- Redis for cache, sessions, idempotency, and runtime flags.
- PostgreSQL/TimescaleDB for trade journal, audit log, and time-series storage.
- Vault or equivalent for secret management.

### Data Storage and Privacy

- Trade and audit data must be stored with tamper-evident linkage.
- Sensitive credentials must not be stored in plaintext.
- PII handling must be minimized and encrypted where required.

### Scalability

- Kubernetes horizontal scaling should be based on concrete signals such as consumer lag, latency, and error rate.
- Go should remain preferred for latency-sensitive services.
- Python should remain preferred for analytics, research, model training, and strategy logic.

## 10. Risks and Challenges

- Exchange API rate limits may delay fills or cancellations.
- Model decay can degrade ML filters if retraining is not governed.
- Kafka/network partitions require strict offset and replay handling.
- Broker adapters must be separately verified before live mode.
- Compliance requirements require legal and operational review before public claims.

## 11. Milestones

### Phase 1 — Core Trading Loop

Market data → feature engine → strategy engine → risk engine → execution engine.

### Phase 2 — Profit/Risk Enhancements

CAFÉ-RC fusion, ML trade filter, dynamic leverage, adaptive trailing stops.

### Phase 3 — AI Studio and Self-Optimization

Natural-language strategy generation, sandbox validation, shadow deployment, and controlled promotion.

### Phase 4 — Production Hardening

Kubernetes, Helm, monitoring, SHARS, security audit, compliance evidence, and operational runbooks.

## 12. User Stories

### GH-001 — Deploy Platform in Paper Mode

As a professional trader, I want to deploy the platform with a single command so I can evaluate performance without risking real capital.

Acceptance criteria:

- `make up` starts required services.
- System defaults to paper mode.
- Synthetic or configured market data flows through the pipeline.
- Paper orders do not reach an exchange.
- Dashboard shows portfolio and agent state after startup.

### GH-002 — Generate Strategy from Natural Language

As a trader, I want to describe a trading idea in plain English and have the platform generate a backtested strategy candidate.

Acceptance criteria:

- User enters a strategy description.
- System generates reviewable Python strategy code.
- Sandbox backtest runs and displays metrics.
- Passing strategies require explicit approval.
- Failed strategies show rejection reasons.

### GH-003 — Halt All Trading with Kill Switch

As a trader, I want to stop all trading and cancel open orders in an emergency.

Acceptance criteria:

- Kill switch requires deliberate hold or slide confirmation.
- Activation sets a circuit-breaker flag.
- New signals are rejected until reset.
- Open-order cancellation is initiated across configured venues.
- Fallback path exists when WebSocket is unavailable.

### GH-004 — Monitor Risk Limits in Real Time

As a trader, I want to view risk status and receive alerts near limits.

Acceptance criteria:

- Risk page displays daily loss, drawdown, exposure, and risk-per-trade.
- Service panels use green/yellow/red states.
- Warnings display before hard limits are reached.
- Risk parameters are editable only within safe bounds.
- Paper/live mode is clearly visible.

### GH-005 — View Agent Swarm Performance

As a trader, I want to see which agents are active and how they are performing.

Acceptance criteria:

- Agent panel lists active, paused, shadow, error, and auto-paused states.
- Each agent shows PnL, win rate, Sharpe, and trade count where available.
- Auto-paused agents show reason.
- Agent detail view shows performance history and parameters.

### GH-006 — View Live Trading Terminal with Charts

As a trader, I want a professional charting interface with risk-aware order entry.

Acceptance criteria:

- Terminal shows candlestick chart and timeframe selector.
- Symbol selector supports configured assets.
- Overlays include VWAP, session levels, PDH/PDL, and ATR bands.
- Order entry shows slippage, fill probability, and risk approval state.
- Manual orders pass through the same risk validation path.

### GH-007 — Self-Healing Recovery

As a system operator, I want failed services to recover automatically where safe.

Acceptance criteria:

- Health monitor checks configured services.
- Recovery attempts are staged and limited.
- Escalation occurs after repeated failed recovery attempts.
- Recovery events are visible in logs/monitoring.

### GH-008 — Verify Audit Trail Integrity

As a trader, I want to verify that the audit trail is tamper-evident.

Acceptance criteria:

- Audit verification script checks chain integrity.
- Broken chain exits non-zero with corrupted record information.
- Intact chain exits zero with a clear confirmation.

## 13. Production Readiness Caveat

This PRD defines the desired product. It is not proof that the repository currently satisfies every requirement. A requirement is considered complete only when code, tests, deployment configuration, and validation evidence exist in the repo.