#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-omega-prime}"

kubectl apply -f infrastructure/kubernetes/base/namespace.yaml
kubectl -n "$NAMESPACE" apply -f infrastructure/kubernetes/canary/deployment-canary.yaml
kubectl -n "$NAMESPACE" apply -f infrastructure/kubernetes/canary/virtualservice.yaml

echo "Canary deployment applied (90/10 split via Istio VirtualService)."
