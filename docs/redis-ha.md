# Redis HA — ΩMEGA PRIME Δ

Topology (docker-compose.yml):

- `redis` — master (`redis:7.2.5-alpine`, port 6379).
- `redis-replica` — async replica (`--replicaof redis 6379`).
- `redis-sentinel` — Sentinel on 26379 watching `omegamaster`
  (`infrastructure/redis/sentinel.conf`, quorum 1).

The replica is usable TODAY as a read source (e.g. operational dashboards);
Sentinel performs failover elections on master loss.

## ⛔ HONEST CAVEAT — read before relying on this

**Automatic client failover does NOT happen.** Our Go and Python services connect
via plain `redis://redis:6379` URLs with non-Sentinel-aware clients. If Sentinel
promotes the replica, application clients keep talking to the dead master until
a human intervenes. This stack therefore has **failover election without client
failover** — better than nothing (a standby exists, promotion is automated),
but it is NOT transparent HA. Do not present it as such.

What a future change would need for real client failover:

1. Go services: use a Sentinel-aware client (e.g. `rueidis` with
   `ClientOption.InitAddress` pointed at the sentinels, or go-redis
   `NewFailoverClient`), resolving the current master via
   `SENTINEL get-master-addr-by-name omegamaster`.
2. Python services: `redis.sentinel.Sentinel([("redis-sentinel", 26379)])`
   → `sentinel.master_for("omegamaster")` instead of `redis://` URLs.
3. Config: replace `REDIS_URL` with `REDIS_SENTINEL=redis-sentinel:26379` +
   `REDIS_MASTER_NAME=omegamaster`.
4. Production: 3 sentinels on separate hosts, quorum 2 (this compose ships 1
   sentinel / quorum 1 — a single sentinel cannot reach majority with itself
   any other way; it is documented, not hidden).

## Operator procedure — master failure (current, manual)

1. `docker compose exec redis-sentinel redis-cli -p 26379 SENTINEL get-master-addr-by-name omegamaster`
   → note the promoted host (expect `redis-replica`).
2. Confirm the old master is really dead, not partitioned (see chaos-drills.md).
3. Re-point clients: this currently means restarting the stack against the new
   master (or, for a planned switchover, promote manually:
   `SENTINEL failover omegamaster` then restart clients).
4. Rebuild the failed node as the new replica and re-verify
   `redis-cli -p 26379 SENTINEL masters`.

Single-use approval IDs (Redis SETNX) live on the master; on failover, in-flight
single-use state may be lost — the execution engine must treat an unknown
approval ID as suspect (fail closed), never as fresh. This matches the
audit-or-no-trade rule: uncertainty refuses, it does not proceed.
