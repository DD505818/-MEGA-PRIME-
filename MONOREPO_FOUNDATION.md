# ΩMEGA PRIME Δ — Production Monorepo Foundation

Status: scaffold foundation
Default mode: PAPER
Live trading: disabled unless explicitly configured, audited, and approved.

## Core Rule

Frontend is for interaction. Backend is for authority. Risk, auth, encryption, execution, and audit logic must never depend on frontend trust.

## Target Structure

```text
apps/
  web/                 # Next.js/React institutional terminal
  api-gateway/         # HTTP API, auth, command validation
services/
  websocket-gateway/   # realtime portfolio/risk/execution streams
  mcp-gateway/         # AI tool gateway, risk-gated
  risk-service/        # AEGIS 14-gate risk governor
  execution-service/   # VULTURE paper execution FSM
packages/
  shared-types/        # TypeScript contracts
  schemas/             # JSON schemas / validation contracts
adapters/
  brokers/mock/        # paper/sandbox broker adapter
infra/
  nginx/               # reverse proxy config
db/
  schema.sql           # starting relational schema
scripts/
  health-check.sh      # local stack verification
docs/
  architecture.md
  risk-doctrine.md
  event-schemas.md
  mcp-tools.md
```

## Safety Boundary

- PAPER mode is default.
- Mock adapters are used until broker certification exists.
- MCP tools cannot execute live trades directly.
- Every command must flow through auth, permission checks, risk validation, execution, and audit.
