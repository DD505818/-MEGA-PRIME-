# ΩMEGA PRIME Δ — Deployment Design (local Compose, PAPER-only)

**Status: DESIGN ONLY — not adopted, not deployed.** The PAPER deployment target
remains Devon's undecided item (local Docker Compose is the standing
recommendation). This document describes how the full stack *would* run; it
starts nothing and changes no doctrine.

Governing state: **EDGE NOT PROVEN · LIVE LOCKED · PAPER-first, Kraken BTC/USD.**

## 1. What this deploys

One `docker compose up` on a dedicated always-on Linux host: the complete
ΩMEGA PRIME Δ stack in PAPER mode, with the doctrine-hardened web UI as the
read-only operator surface. Postgres and Redis stay private to the Compose
network. No LIVE credentials exist anywhere in this design.

## 2. Services

| Service | Image / build | Port | Role |
| --- | --- | --- | --- |
| zookeeper | confluentinc/cp-zookeeper:7.5.0 | 2181 (internal) | Kafka coordination |
| kafka | confluentinc/cp-kafka:7.5.0 | 9092 | Event bus |
| redis | redis:7.2.5-alpine | 6379 | Dedup, state, audit-halt flags |
| postgres | postgres:16.4-alpine | 5432 | TruthCore chain, portfolio, MIDAS registry |
| market-ingestion | ./apps/market-data-service (Rust) | — | Kraken data ingest (PAPER) |
| feature-engine | ./apps/feature-engine | — | Feature computation |
| strategy-engine | ./apps/agent-service (Python) | — | AGENTS + MIDAS promotion gate |
| fusion-engine | ./apps/fusion-engine | 8085 | Signal fusion |
| risk-engine | ./apps/risk-service (Go) | 8080 | AEGIS Governor — 14 gates, audit-or-no-trade |
| execution-engine | ./apps/execution-service (Go) | 8081 | VULTURE — PAPER fills only, read-only `/orders` |
| capital-allocator | ./apps/capital-allocator | 8082 | Kelly allocation (gated) |
| portfolio-service | ./apps/portfolio-service | 8083 | Positions/accounting (PAPER) |
| truth-core | ./apps/truth-core (Go) | 8084 | Hash-chained audit, append-only |
| llm-service | ./apps/llm-service | — | Research assistance — **never an execution authority** |
| websocket-gateway | ./apps/websocket-gateway | 3001 | Subscribe-only market stream |
| web-ui | ./apps/web-ui (Next.js 14) | 3000 | Operator dashboard — **observe/explain/request/display only** |

## 3. Environment contract (modelock)

The canonical mode contract lives in `apps/modelock/` (Go) and is wired into
all six Go services' startup, the Rust market-data service, and the Python
agent-service:

- `PAPER_MODE=true` is the **only** accepted PAPER signal.
- `LIVE_TRADING_ENABLED=true` vetoes unconditionally.
- Unset, empty, or malformed mode fails closed.
- `LiveLocked=true` at every startup gate.

Compose defaults (`docker-compose.yml`): `PAPER_MODE: ${PAPER_MODE:-true}`,
`LIVE_TRADING_ENABLED: ${LIVE_TRADING_ENABLED:-false}` on every gated service.
`.env.example` ships `PAPER_MODE=true`.

Fail-fast secrets (Compose refuses to boot without them — no fallbacks):
`AEGIS_APPROVAL_PRIVKEY` (generate: `go run ./apps/approval/cmd/keygen`),
`AEGIS_APPROVAL_PUBKEY` (must match), `TRUTHCORE_WRITE_SECRET` (must match
truth-core's), `JWT_SECRET`, `POSTGRES_PASSWORD`. No credentials are committed
anywhere; the dashboard never handles them.

## 4. Volumes and data

- `pgdata` — Postgres data (private).
- `${EVIDENCE_HOST_DIR:-./evidence}:/evidence:ro` — **read-only** mount feeding
  the dashboard Evidence Center. Expected layout:
  `campaign_summary.json`, `campaign2_summary.json`,
  `reports-c1/*.json`, `reports-c2/*.json`.
  When the mount is absent, the dashboard renders UNKNOWN — it never fabricates
  campaign evidence.

## 5. Health and readiness

Every stateful or serving component has a healthcheck (infra: `nc`/`redis-cli`/
`pg_isready`/broker API versions; web-ui: `GET /health/ready` via the
Dockerfile `HEALTHCHECK`). Gated services `depends_on` healthy infra;
`restart: unless-stopped` everywhere.

## 6. Bring-up (when Devon adopts a target)

```bash
cp .env.example .env
go run ./apps/approval/cmd/keygen          # AEGIS_APPROVAL_PRIVKEY / _PUBKEY
export AEGIS_APPROVAL_PRIVKEY=...          # + the other fail-fast secrets
export AEGIS_APPROVAL_PUBKEY=...
export TRUTHCORE_WRITE_SECRET=...
export JWT_SECRET=...
export POSTGRES_PASSWORD=...
# Optional: point the dashboard at campaign artifacts (read-only)
export EVIDENCE_HOST_DIR=/path/to/campaign-artifacts
docker compose up --build -d
docker compose ps                            # all services healthy
open http://localhost:3000                   # operator dashboard
```

Shutdown: `docker compose down` (add `-v` only if you intend to drop `pgdata`).

## 7. LIVE is architecturally unreachable

This section is load-bearing, not boilerplate.

- Audited 2026-09-30 across the merged tree: **no order-submission path exists
  anywhere** — execution `/orders` is read-only, the gateway is subscribe-only,
  the web UI is GET-only stubs, there is no CLI entrypoint and no broker client.
- The modelock contract vetoes any LIVE mode unconditionally; `LiveLocked=true`.
- The dashboard cannot place or edit orders, cannot trigger the kill switch,
  cannot edit risk limits, and cannot fabricate balances, fills, P&L, positions,
  risk state, or service health. Kill status is displayed read-only; limit
  changes require the backend/operator path outside this interface.
- Any future LIVE activation belongs to the **separate independent
  certification program** and is out of scope for this interface. Required
  wording, shown on the dashboard banner: *"Requires independent certification
  and backend governance outside this interface."*

## 8. What this design does NOT do

- Does not acquire the tick archive or any richer Campaign #3 data (needs
  Devon's explicit authorization).
- Does not merge PR #75 (MC v2, unmerged) — the dashboard notes its status as
  "in review (PR #75, unmerged)".
- Does not start PAPER campaigns (gated on historical qualification), SHADOW,
  or any trading.
- Does not select the deployment target — that decision is Devon's.
