# ΩMEGA PRIME v9 – Advanced Production Add-ons

This document tracks the implementation scaffolding for three advanced add-ons:

1. Kafka event sourcing (`services/event_sourcing/`)
2. FIX 4.4 gateway (`services/fix_gateway/`)
3. Multi-region active-active Redis (`backend/shared/utils/active_active_redis.py`)

## Integration notes

- Kafka producers/consumers are designed for replayable order/fill/risk streams.
- FIX gateway code is isolated in its own service folder and can run as a separate container.
- Active-active Redis wrapper supports write fan-out and read failover across configured regions.

## Deployment resources

- Redis active-active CRDB manifest: `infrastructure/kubernetes/redis-aa.yaml`
- Multi-region Redis compose simulation: `infrastructure/docker-compose.redis-aa.yml`
