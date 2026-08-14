apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-managed-kafka
  namespace: omega-prime
spec:
  podSelector:
    matchExpressions:
      - key: app.kubernetes.io/name
        operator: In
        values:
          - market-data-service
          - feature-engine
          - agent-service
          - fusion-engine
          - risk-service
          - execution-service
          - capital-allocator
          - portfolio-service
          - websocket-gateway
          - llm-service
  policyTypes: [Egress]
  egress:
    - to: [{ipBlock: {cidr: "${KAFKA_EGRESS_CIDR}"}}]
      ports: [{protocol: TCP, port: ${KAFKA_EGRESS_PORT}}]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-managed-redis
  namespace: omega-prime
spec:
  podSelector:
    matchExpressions:
      - key: app.kubernetes.io/name
        operator: In
        values:
          - feature-engine
          - fusion-engine
          - risk-service
          - execution-service
          - capital-allocator
          - portfolio-service
          - websocket-gateway
          - llm-service
  policyTypes: [Egress]
  egress:
    - to: [{ipBlock: {cidr: "${REDIS_EGRESS_CIDR}"}}]
      ports: [{protocol: TCP, port: ${REDIS_EGRESS_PORT}}]
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-managed-postgres
  namespace: omega-prime
spec:
  podSelector:
    matchExpressions:
      - key: app.kubernetes.io/name
        operator: In
        values: [portfolio-service, truth-core]
  policyTypes: [Egress]
  egress:
    - to: [{ipBlock: {cidr: "${POSTGRES_EGRESS_CIDR}"}}]
      ports: [{protocol: TCP, port: ${POSTGRES_EGRESS_PORT}}]
