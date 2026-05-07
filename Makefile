.PHONY: up down test deploy-staging deploy-prod chaos verify

up:
	cp -n .env.example .env 2>/dev/null || true
	docker-compose up -d --build
	@echo "→ Web UI:    http://localhost:3000"
	@echo "→ Risk API:  http://localhost:8080/status"
	@echo "→ WS:        ws://localhost:3001/ws?token=dev-token"

down:
	docker-compose down -v

logs:
	docker-compose logs -f --tail=100

verify:
	bash scripts/verify_10_of_10.sh

test:
	@echo "--- Python agent tests ---"
	cd apps/agent-service && python -m pytest ../../tests/ -v --tb=short
	@echo "--- Go risk-service tests ---"
	cd apps/risk-service && go test ./... -v
	@echo "--- TypeScript type check ---"
	cd apps/web-ui && npx tsc --noEmit
	@echo "--- System verification ---"
	bash scripts/verify_10_of_10.sh

test-agents:
	cd apps/agent-service && python -m pytest ../../tests/ -v

deploy-staging:
	kubectl config use-context staging-cluster
	helm upgrade --install omega infra/helm/omega -f infra/helm/omega/values.yaml

deploy-prod:
	kubectl config use-context prod-cluster
	helm upgrade --install omega infra/helm/omega -f infra/helm/omega/values.yaml

chaos:
	bash scripts/run_chaos.sh

kill:
	curl -s -X POST http://localhost:8080/kill -H 'Content-Type: application/json' \
	     -d '{"reason":"manual-operator"}' | jq .

reset-kill:
	curl -s -X POST http://localhost:8080/reset | jq .

status:
	curl -s http://localhost:8080/status | jq .
	curl -s http://localhost:8083/portfolio | jq .
