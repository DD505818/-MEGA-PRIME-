# Monitoring

## Service health checks

- `market-ingestion`: `GET /health`
- `portfolio-service`: `GET /health`
- `alpha_factory`: `GET /health`
- `impact_model`: `GET /health`
- `meta_controller`: `GET /health`
- `price_predictor`: `GET /health`
- Frontend nginx: `GET /health`

## Log monitoring

All new bootstrap services emit structured logs.

Alert on:
- startup configuration validation failures,
- repeated ingestion/reconciliation loop failures,
- non-graceful shutdowns,
- sustained 5xx responses from ML inference endpoints.

## CI placeholder guard

`omega-prime-delta/scripts/check-placeholders.sh` is enforced by GitHub Actions.
Any sentinel placeholder in bootstrap-critical files fails CI immediately.
