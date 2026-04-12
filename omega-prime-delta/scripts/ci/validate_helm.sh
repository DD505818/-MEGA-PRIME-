#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHART_DIR="$ROOT_DIR/infrastructure/helm/omega-prime-delta"

required=(
  "$CHART_DIR/Chart.yaml"
  "$CHART_DIR/values.yaml"
  "$CHART_DIR/templates/deployments.yaml"
  "$CHART_DIR/templates/services.yaml"
)

for file in "${required[@]}"; do
  if [[ ! -f "$file" ]]; then
    echo "[helm-gate] ERROR: missing file $file"
    exit 1
  fi
done

grep -q '^apiVersion: v2' "$CHART_DIR/Chart.yaml" || { echo "[helm-gate] ERROR: Chart.yaml must declare apiVersion v2"; exit 1; }
grep -q '^name: omega-prime-delta' "$CHART_DIR/Chart.yaml" || { echo "[helm-gate] ERROR: Chart.yaml name mismatch"; exit 1; }
grep -q '^services:' "$CHART_DIR/values.yaml" || { echo "[helm-gate] ERROR: values.yaml must define services"; exit 1; }

echo "[helm-gate] helm chart structure validated"
