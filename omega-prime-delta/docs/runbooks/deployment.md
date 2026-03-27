# Deployment

1. Export environment values from `.env.example`.
2. Start dependencies with `docker compose -f infrastructure/docker-compose.yml up -d postgres kafka`.
3. Apply DB schema with `psql "$DATABASE_URL" -f backend/db/schema.sql`.
4. Start services (`risk-engine`, `capital-allocator`, `execution-engine`, `strategy-engine`).
