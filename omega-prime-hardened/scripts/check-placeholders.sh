#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

patterns=(
  'TODO'
  'implement module logic'
  'requirements placeholder'
)

files=(
  backend/cmd/market-ingestion/main.go
  backend/cmd/portfolio-service/main.go
  backend/internal/brokers/*.go
  ml/*/service.py
  ml/*/train.py
  ml/*/requirements.txt
  ml/requirements.txt
  frontend/Dockerfile
  frontend/nginx.conf
  frontend/tailwind.config.js
  README.md
  docs/runbooks/deployment.md
  docs/runbooks/monitoring.md
)

fail=0
for pattern in "${patterns[@]}"; do
  if rg -n --ignore-case "$pattern" "${files[@]}"; then
    echo "Found forbidden placeholder pattern: $pattern"
    fail=1
  fi
done

if [[ $fail -ne 0 ]]; then
  echo "Placeholder guard failed."
  exit 1
fi

echo "Placeholder guard passed."
