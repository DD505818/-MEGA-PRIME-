# PAPER/LIVE Lock — Phase 2 program

Freeze baseline: `235ff5f` (tag `freeze/2026-09-29-program-start`).
Branch: `phase/2-paper-live-lock`.

Governing constraint: **LIVE IS LOCKED.** There is no code path in this
repository that can enable live broker submission, and no supported
configuration that selects LIVE mode, until a separate certification program
explicitly authorizes changing that constraint.

## Canonical mode contract

One interpreter, four implementations (same rule):

| Implementation | Location |
|---|---|
| Go | `apps/modelock/` (`IsPaper`, `RequirePaper`, `AssertPaper`) |
| Rust | `require_paper()` in `apps/market-data-service/src/main.rs` |
| Python | `require_paper()` in `apps/agent-service/modelock.py` |
| Docs/CI | this file; `.env.example`; `docker-compose.yml`; k8s `config.yaml` |

Rule:

- `PAPER_MODE=true` (case-insensitive, trimmed) → paper. The only runnable mode.
- `LIVE_TRADING_ENABLED=true` → hard refusal, unconditionally, even with `PAPER_MODE=true`.
- Anything else — unset, empty, malformed — → **not paper → fail closed.**

Previously the services interpreted the env inconsistently
(execution-service treated empty as paper; risk-service Gate 3 treated empty
as a live environment). That inconsistency is removed: every service now
calls the canonical check.

## Path audit

| Path | Finding | Lock |
|---|---|---|
| **Startup** (Go ×6) | Only execution-service checked mode; others started unconditionally | `modelock.RequirePaper` is now the first statement in `main()` of execution, risk, portfolio, capital-allocator, fusion, truth-core. Any non-paper env → `log.Fatal` before consumers/servers start |
| **Startup** (Rust market-data) | No mode check | `require_paper()` at top of `main()`; `exit(1)` otherwise |
| **Startup** (Python agent-service) | No mode check | `require_paper("agent-service")` in `main.py` before the orchestrator runs |
| **Worker** (execution Kafka consumer, `signals.approved`) | Mode read once at startup; `liveFill` branch reachable in theory | `modelock.AssertPaper()` on **every** `processSignal`; refusal cancels the order with `MODELOCK_REFUSAL` and audits it |
| **Broker** | `liveFill` silently delegated to `paperFill` ("broker integration point") | `liveFill` now returns a hard error: `LIVE execution is locked`. Dead TWAP/Iceberg simulation helpers removed |
| **API** | `execution-service /orders` is read-only (lists in-memory orders); no order-submission HTTP endpoint exists in any service | Verified; nothing to gate |
| **Database** | portfolio-service consumes `orders.fills` and maintains paper positions in Redis/Postgres | Startup-gated by `RequirePaper`; no live-position path exists |
| **CLI** | No CLI entrypoint exists in the repo (no cobra/urfave, no `cmd/` trees, scripts are build/verify/token-mint) | Verified absent |
| **Webhook** | websocket-gateway has no POST/webhook handlers; it subscribes read-only to `orders.fills`, `orders.routed` | Verified read-only |
| **UI** | web-ui API routes are GET-only stubs (`{data: []}`); no fetch/POST in `src/` | Verified observe-only |

## Gate 3 (risk-service)

Rewritten against the canonical lock:

- Process not in paper mode → `GATE3_NOT_IN_PAPER_MODE` (defense in depth;
  unreachable post-`RequirePaper`, but unit-testable).
- Signal declares a non-paper `mode` → `GATE3_SIGNAL_MODE_MISMATCH`.
- Empty/missing signal mode (legacy) → passes Gate 3.

## Tests

- `apps/modelock/modelock_test.go` — 14-case env matrix (paper/live-flag
  combinations, case/whitespace variants) for `IsPaper`/`AssertPaper`, plus
  `LiveLocked` constant guard.
- `apps/execution-service/modelock_test.go` — `liveFill` errors for every
  order type and returns zero price; contract test for the env rule.
- `apps/risk-service/modelock_test.go` — live/`LIVE`/`Live`/padded/`REAL`
  signal modes rejected with `GATE3_SIGNAL_MODE_MISMATCH`; paper and empty
  modes not rejected by Gate 3.
- Full suites: risk-service 73 tests, execution-service, modelock — all green.

## Residual notes (not lock defects)

- `LIVE_TRADING_ENABLED=true` can never enable trading — it only kills
  startup faster. There is no live broker client anywhere in the repo
  (verified: no exchange API keys/secrets; market-data uses the public
  Kraken ticker only).
- `feature-engine` and `llm-service` (Python) have no order or position
  capability (features/text only); they are not startup-gated. If either
  gains execution-adjacent capability, it must adopt `modelock.py`.
