# Deployment

## Prerequisites

- Docker and Docker Compose
- Python 3.11+ for ML services
- Go 1.22+ for backend services

## Bootstrap deployment sequence

1. **Bring up shared dependencies**
   - `docker compose -f infrastructure/docker-compose.yml up -d postgres redis kafka`
2. **Initialize database**
   - `psql "$DATABASE_URL" -f backend/db/schema.sql`
3. **Deploy backend bootstrap services**
   - `go run backend/cmd/market-ingestion/main.go`
   - `go run backend/cmd/portfolio-service/main.go`
4. **Deploy ML services**
   - Install per-service dependencies from each `ml/*/requirements.txt`.
   - Start services:
     - `python ml/alpha_factory/service.py`
     - `python ml/impact_model/service.py`
     - `python ml/meta_controller/service.py`
     - `python ml/price_predictor/service.py`
5. **Deploy frontend**
   - Build with `docker build -t omega-prime-delta-frontend frontend`.
   - Run container and verify `/health` returns `ok`.

## Required environment variables

- `MARKET_SOURCE`
- `PORTFOLIO_BASE_CURRENCY`
- `ALPHA_MODEL_REGISTRY`
- `IMPACT_COEFF`
- `IMPACT_TRAIN_DATASET`
- `META_CONTROLLER_REPLAY_PATH`
- `PRICE_TRAIN_DATASET`

Set optional overrides for ports and intervals as needed.
