# ΩMEGA PRIME Δ Kubernetes production layer

## Safety contract

This deployment is PAPER-only. The checked-in ConfigMap forces `PAPER_MODE=true`,
`TRADING_MODE=PAPER`, and `LIVE_TRADING_ENABLED=false`. Do not enable live trading
through an overlay. Live enablement requires a separately reviewed release, broker
certification, reconciliation tests, and a tested kill switch.

## Prerequisites

- Kubernetes 1.29+
- Kustomize 5+
- metrics-server for HPAs
- a NetworkPolicy-capable CNI
- ingress-nginx if exposing the WebSocket gateway
- managed Kafka, Redis, and PostgreSQL with TLS
- immutable images published for commit `db1f614a8cc6a76326434f10be49b590d2afbba8`

The repository currently has no Dockerfile for `apps/web-ui`; therefore this layer
does not pretend the UI is deployable. Add and verify an image build before adding it.

## Required secret

Create `omega-production` out of band. Never apply the example unchanged.

```bash
kubectl create namespace omega-prime --dry-run=client -o yaml | kubectl apply -f -
kubectl -n omega-prime create secret generic omega-production \
  --from-literal=KAFKA_BROKERS='...' \
  --from-literal=REDIS_URL='rediss://...' \
  --from-literal=POSTGRES_DSN='postgresql://...?sslmode=require' \
  --from-literal=JWT_SECRET='...'
```

## Validate and deploy

```bash
kubectl kustomize infra/kubernetes/overlays/production > /tmp/omega-production.yaml
kubectl apply --dry-run=server -f /tmp/omega-production.yaml
kubectl apply -k infra/kubernetes/overlays/production
kubectl -n omega-prime rollout status deployment --timeout=5m
```

## Production gates

1. Images exist for the pinned Git SHA and pass vulnerability/SBOM/signature checks.
2. Secret values use TLS endpoints and are supplied by an external secret manager.
3. NetworkPolicy egress is narrowed to the actual managed-service CIDRs.
4. TCP probes are replaced by semantic `/health/live` and `/health/ready` endpoints.
5. One deterministic PAPER flow reaches TruthCore and survives restart/reconciliation.
6. LIVE remains disabled.
