.PHONY: up down test lint db-migrate

up:
	docker compose -f infrastructure/docker-compose.yml up -d --build

down:
	docker compose -f infrastructure/docker-compose.yml down -v

test:
	cd backend && go test ./...

lint:
	cd backend && go vet ./...

db-migrate:
	docker compose -f infrastructure/docker-compose.yml run --rm postgres psql -h postgres -U omega -d omega -f /docker-entrypoint-initdb.d/002_audit_immutable.sql
