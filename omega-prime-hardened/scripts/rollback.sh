#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-omega-prime}"

kubectl -n "$NAMESPACE" rollout undo deployment/execution-engine-stable
kubectl -n "$NAMESPACE" scale deployment/execution-engine-canary --replicas=0

echo "Rollback complete: stable restored, canary scaled down."
