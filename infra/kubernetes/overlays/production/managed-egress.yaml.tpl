apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-managed-data-services
  namespace: omega-prime
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/part-of: omega-prime
  policyTypes: [Egress]
  egress:
    - to: [{ipBlock: {cidr: "${KAFKA_EGRESS_CIDR}"}}]
      ports: [{protocol: TCP, port: 9093}]
    - to: [{ipBlock: {cidr: "${REDIS_EGRESS_CIDR}"}}]
      ports: [{protocol: TCP, port: 6379}]
    - to: [{ipBlock: {cidr: "${POSTGRES_EGRESS_CIDR}"}}]
      ports: [{protocol: TCP, port: 5432}]
