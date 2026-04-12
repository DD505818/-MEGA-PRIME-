# OMEGA Prime A Hardening Blueprint

This document captures the hardening targets for OMEGA Prime A and maps them to repository artifacts.

## Deployment controls

- Canary deployment manifests with Istio traffic splitting live under `infrastructure/kubernetes/canary/`.
- Rollback and schema migration scripts are in `scripts/` and are wired into GitHub Actions workflows.

## CI/CD workflows

- `.github/workflows/ci.yml`: baseline Go + Python checks.
- `.github/workflows/cd-canary.yml`: manual canary deployment with rollback on gate failure.
- `.github/workflows/schema-migrate.yml`: manual, controlled migration direction (`up`/`down`).

## Observability alerts

- `infrastructure/monitoring/prometheus/alerts.yml` defines slippage, fill-rate, reject-rate, and latency alerts.

## Operational notes

- This blueprint is intentionally conservative and incremental; deeper execution/risk engine hardening should be implemented with integration tests before enabling live order flow.
