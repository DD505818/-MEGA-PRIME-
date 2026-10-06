# Chaos drills — ΩMEGA PRIME Δ (PAPER stack)

Cadence: **weekly**. Scope: the local PAPER compose stack ONLY. Never against
anything holding real funds, real credentials, or production data — there is no
such thing in this program (LIVE is architecturally unreachable), and these
drills must never be the thing that creates one.

General rules:

- Drills run in PAPER mode with the modelock verified (`PAPER_MODE=true`,
  `LIVE_TRADING_ENABLED` unset/false). If the mode contract does not verify,
  the drill aborts before injection.
- One drill per session. Record evidence per the template below; file it under
  `docs/chaos-evidence/` (gitignored — evidence is local, not committed).
- Abort criteria are checked BEFORE each injection step, not just at the start.

## Drill 1 — Kill the Kafka leader

- **Hypothesis:** producers fail closed (no silent order loss); consumers resume
  from committed offsets; no duplicate `orders.approved` is acted on twice
  (idempotency + single-use approval IDs hold).
- **Blast radius:** event bus only. No state loss beyond unflushed producer
  buffers (document the count).
- **Procedure:** `docker compose kill -s SIGKILL kafka` → wait 60s → `docker compose up -d kafka`.
- **Abort if:** modelock not PAPER; any service already unhealthy pre-injection.
- **Expected:** risk-engine rejects proposals while bus is down (fail-closed);
  execution emits nothing; on recovery, lag drains, offsets resume, TruthCore
  shows a gap in `audit.events` with no orphaned approvals.

## Drill 2 — Kill the Postgres primary

- **Hypothesis:** TruthCore writes fail closed (audit-or-no-trade: execution
  refuses fills it cannot audit); reads served from `postgres-replica`;
  no split-brain writes.
- **Blast radius:** truth/audit persistence. In-flight fills must halt, not proceed unaudited.
- **Procedure:** `docker compose kill -s SIGKILL postgres` → observe 120s →
  `docker compose up -d postgres` → verify replica re-sync
  (`pg_stat_replication` on primary, replay lag ≈ 0).
- **Abort if:** replica lag > 5 min pre-injection; modelock not PAPER.
- **Expected:** `AUDIT_UNAVAILABLE` refusals in execution-service logs; zero
  fills during the outage; after recovery, reconciliation reports the outage
  window explicitly (no backfilled silence).

## Drill 3 — Network-partition the risk engine

- **Hypothesis:** partition is indistinguishable from risk-engine death:
  strategy proposals get no approval, execution gets no new approvals,
  in-flight approvals expire via `expires_at_unix` / kill-epoch checks.
- **Blast radius:** authorization path only.
- **Procedure:** `docker network disconnect <compose-net> risk-engine`
  (or iptables DROP on its port) → hold 90s → reconnect.
- **Abort if:** any `orders.approved` younger than 60s is unaccounted for.
- **Expected:** no approvals issued during partition (nothing to issue them
  with); execution's single-use/expiry checks reject anything stale on
  recovery; kill-epoch mismatch → reject, never retry-as-fresh.

## Drill 4 — Disk pressure on Redis

- **Hypothesis:** with `maxmemory 512mb allkeys-lru`, Redis evicts rather than
  OOMs; single-use approval SETNX degrades to "unknown" → execution fails
  closed on unknown IDs (never treats unknown as fresh).
- **Blast radius:** ephemeral state (approvals, rate-limit buckets).
- **Procedure:** fill Redis to >90% (`DEBUG` or a loader loop — never on a host
  with real data) → attempt a full propose→approve→execute cycle → drain.
- **Abort if:** eviction policy is not `allkeys-lru` (check `CONFIG GET maxmemory-policy`).
- **Expected:** approval cycle refused or fully traceable; no approval executed
  twice; no silent acceptance of an evicted single-use key.

## Evidence log template

```markdown
# Chaos drill evidence — <DRILL-ID>
- Date (UTC):
- Operator:
- Drill: 1|2|3|4 (name)
- Mode verification (pre): PAPER_MODE=… LIVE_TRADING_ENABLED=… (attach)
- Injector (exact command):
- Observed behavior:
- Expected behavior (per runbook):
- Match? YES / NO (deviations listed):
- TruthCore / audit refs (entry hashes or time range):
- Recovery time (inject → all healthchecks green):
- Follow-up issues filed:
```
