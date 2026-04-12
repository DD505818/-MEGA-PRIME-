#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-omega-prime}"
K8S_DIR="infrastructure/kubernetes"

echo "Applying base resources to namespace ${NAMESPACE}"
kubectl apply -f "${K8S_DIR}/base/namespace.yaml"
kubectl apply -f "${K8S_DIR}/base/storage.yaml"

echo "Applying stateful services"
kubectl apply -f "${K8S_DIR}/services/postgres.yaml"
kubectl apply -f "${K8S_DIR}/services/redis.yaml"
kubectl apply -f "${K8S_DIR}/services/kafka.yaml"
kubectl apply -f "${K8S_DIR}/services/clickhouse.yaml"

echo "Applying deployments"
for manifest in "${K8S_DIR}"/deployments/*.yaml; do
  kubectl apply -f "${manifest}"
done

echo "Applying ingress and monitoring stack"
kubectl apply -f "${K8S_DIR}/services/ingress.yaml"
kubectl apply -f "${K8S_DIR}/monitoring/prometheus.yaml"
kubectl apply -f "${K8S_DIR}/monitoring/alertmanager.yaml"
kubectl apply -f "${K8S_DIR}/monitoring/grafana.yaml"

echo "Deployment completed"
