.PHONY: deps-go deps-py deps-npm proto validate

deps-go:
	cd backend && go mod tidy

deps-py:
	python3 -m pip freeze > backend/cmd/strategy-engine/requirements.txt
	python3 -m pip freeze > ml/requirements.txt

deps-npm:
	cd frontend && npm install --package-lock-only
	cd backend/cmd/websocket-gateway && npm install --package-lock-only

proto:
	@echo "No .proto files found; skipping Protobuf code generation."

validate:
	test -f docker-compose.yml
	test -f Makefile
	test -f openapi.yaml
