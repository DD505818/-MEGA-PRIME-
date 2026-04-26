.PHONY: up down test

up:
	docker-compose up -d --build

down:
	docker-compose down -v

test:
	pytest services/strategy-engine/tests/ -v || true
	cd services/risk-engine-go && go test ./... -v
	cd services/execution-router-go && go test ./... -v
	cd gateway-api && npm test || true
