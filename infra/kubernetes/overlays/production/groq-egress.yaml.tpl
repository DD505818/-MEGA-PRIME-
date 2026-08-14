apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-managed-groq
  namespace: omega-prime
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: llm-service
  policyTypes: [Egress]
  egress:
    - to: [{ipBlock: {cidr: "${GROQ_EGRESS_CIDR}"}}]
      ports: [{protocol: TCP, port: 443}]
