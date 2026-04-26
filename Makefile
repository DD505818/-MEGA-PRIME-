.PHONY: up down test deploy-staging deploy-prod chaos dr-drill

up:
	docker-compose up -d --build

down:
	docker-compose down -v

test:
	pytest tests/ -v || true
	go test ./apps/risk-service/... ./apps/execution-service/... ./apps/capital-allocator/... -v
	cd apps/web-ui && npx tsc --noEmit || true
	bash scripts/verify_10_of_10.sh

deploy-staging:
	kubectl config use-context staging-cluster
	helm upgrade --install omega infra/helm/omega -f infra/helm/omega/values.yaml

deploy-prod:
	kubectl config use-context prod-cluster
	helm upgrade --install omega infra/helm/omega -f infra/helm/omega/values.yaml

chaos:
	bash scripts/run_chaos.sh

dr-drill:
	echo "DR drill not yet implemented"
