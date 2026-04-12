# ΩMEGA Prime Δ

This directory contains the runnable baseline for ΩMEGA Prime Δ across backend, ML, and frontend services.

## Implemented bootstrap scope

- **Go services**
  - `market-ingestion`: HTTP health endpoint, ingestion ticker loop, env validation, JSON structured logging, graceful shutdown.
  - `portfolio-service`: HTTP health + portfolio endpoints, reconciliation loop, env validation, JSON structured logging, graceful shutdown.
  - `backend/internal/brokers`: normalized order/fill types and broker adapter constructors for Alpaca, Binance, Coinbase, IBKR, Kraken, MT5, and Oanda.
- **Python ML services**
  - Runnable FastAPI entrypoints for `alpha_factory`, `impact_model`, `meta_controller`, and `price_predictor` with startup env validation and signal-aware shutdown.
  - Runnable training entrypoints for `impact_model`, `meta_controller`, and `price_predictor` that emit training artifacts.
  - Concrete pinned `requirements.txt` files for each ML service and top-level ML dependencies.
- **Frontend container runtime**
  - Multi-stage Docker build, production nginx config with SPA fallback and `/health`, and concrete Tailwind theme config.
- **CI guardrail**
  - GitHub Actions workflow and script to fail changes when sentinel placeholders appear in critical bootstrap files.

- **Delivery contracts and deployment packaging**
  - Helm chart under `infrastructure/helm/omega-prime-delta` for market-ingestion and portfolio-service deployments.
  - Service contracts in `contracts/service-contracts.yaml` and protobuf definitions in `contracts/proto/omega/prime/delta/v1/`.
  - CI gate scripts under `scripts/ci/` enforcing contract, protobuf, and Helm chart integrity in pull requests.


## Quick start

1. Export required environment variables for the service you plan to run.
2. Start dependencies with `docker compose -f infrastructure/docker-compose.yml up -d`.
3. Start backend services from `omega-prime-delta/backend/cmd/...`.
4. Start ML services with `python -m <module>.service` or run training scripts directly.
5. Build and serve the frontend with `frontend/Dockerfile`.


## Hardened controls (April 2026 update)

The repository now includes a hardening baseline for controlled production rollouts:

- Canary deployment and traffic split manifests under `infrastructure/kubernetes/canary/`
- Prometheus alert rules for slippage/fill/reject/latency in `infrastructure/monitoring/prometheus/alerts.yml`
- Operational scripts for canary deploy, rollback, and schema migration in `scripts/`
- GitHub Actions pipelines for CI, canary CD, and schema migration under `.github/workflows/`

See `docs/HARDENING_BLUEPRINT.md` for details.
