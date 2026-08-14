# ΩMEGA PRIME Δ Kubernetes production layer

## Safety contract

This deployment is PAPER-only. The checked-in runtime forces
`PAPER_MODE=true`, `TRADING_MODE=PAPER`, and
`LIVE_TRADING_ENABLED=false`. Execution also refuses to start if PAPER mode
is disabled or LIVE is requested. `BROKER_CERTIFIED` and
`FAILURE_TESTS_PASSED` remain `false`.

## Published image lock

All 12 application images, including the web UI, were built from source
revision `d6bf0eede8efba73106c8ee6578604be7070b81e` and published to flat
`ghcr.io/dd505818/omega-prime-<service>` packages with BuildKit provenance and
SBOM attestations. The Deployments in `base/workloads.yaml` and
`base/workers.yaml` pin those artifacts by OCI digest, not by a mutable tag.

- [GHCR publication run](https://github.com/DD505818/-MEGA-PRIME-/actions/runs/31782713952)
- [Deterministic PAPER proof](https://github.com/DD505818/-MEGA-PRIME-/actions/runs/31782714012)

## Prerequisites

- Kubernetes 1.29+
- Kustomize 5+
- metrics-server for HPAs
- a NetworkPolicy-capable CNI
- ingress-nginx if exposing the WebSocket gateway
- cluster pull access to the published GHCR packages
- a protected GitHub Environment named `paper`
- managed Kafka, Redis, and PostgreSQL endpoints with publicly trusted or
  cluster-installed CA chains

## Protected `paper` Environment

Supply these as GitHub Environment secrets; never commit their values:

- `KAFKA_BROKERS`
- `KAFKA_SASL_USERNAME`
- `KAFKA_SASL_PASSWORD`
- `REDIS_URL`
- `POSTGRES_DSN`
- `JWT_SECRET`
- `GROQ_API_KEY` (optional)
- `KUBE_CONFIG_B64`

The deployment workflow enforces Kafka `SASL_SSL`, requires a
`rediss://` Redis URL, and requires PostgreSQL
`sslmode=verify-full`. It creates dependency-scoped Secrets
(`omega-kafka`, `omega-redis`, `omega-postgres`,
`omega-websocket`, and `omega-llm`) so workloads do not receive unrelated
credentials.

Supply these GitHub Environment variables from the managed-service provider's
current, approved network ranges and ports:

- `KAFKA_EGRESS_CIDR` and `KAFKA_EGRESS_PORT`
- `REDIS_EGRESS_CIDR` and `REDIS_EGRESS_PORT`
- `POSTGRES_EGRESS_CIDR` and `POSTGRES_EGRESS_PORT`
- `GROQ_EGRESS_CIDR` when `GROQ_API_KEY` is set
- `MANAGED_EGRESS_APPROVED=true` after the ranges are independently reviewed

Placeholders, default routes, loopback, multicast, unspecified networks, IPv4
ranges broader than /16, and IPv6 ranges broader than /48 are rejected. The
rendered policies grant each workload only the managed dependencies it uses.

## Health contract

Every production workload exposes:

- `/health/live`: the process and health server are responsive.
- `/health/ready`: required startup dependencies are usable.

Risk, execution, and portfolio readiness additionally require a Kafka consumer
assignment; portfolio also requires Redis and PostgreSQL. Kubernetes uses HTTP
probes with explicit five-second timeouts, and TruthCore has a startup probe for
database initialization.

## Validate and deploy

Render and validate locally:

```bash
kubectl kustomize infra/kubernetes/overlays/production > /tmp/omega-production.yaml
kubectl apply --dry-run=server -f /tmp/omega-production.yaml
```

Deploy only through the manually dispatched
`Deploy Kubernetes PAPER` workflow. It validates TLS inputs and approved
CIDRs, creates the scoped Secrets, renders exact egress policies, deploys the
digest-locked workloads, waits for rollout, and re-checks every PAPER/LIVE gate.

## PAPER evidence and LIVE gate

The deterministic proof pre-creates lifecycle topics, waits for consumer-ready
services, verifies the fail-closed LIVE startup guard, submits a fixed PAPER
signal, checks the deterministic fill, reconciles execution and portfolio state
against PostgreSQL, and validates the six-entry TruthCore chain.

LIVE must remain disabled until a real broker adapter is certified, broker-
authoritative reconciliation and failure injection pass, kill-switch state is
cluster-consistent, and the release gates receive a separate reviewed change.
