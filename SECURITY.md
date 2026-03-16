# Security Hardening

- Enable mTLS between services via service mesh (Istio/Linkerd).
- Rotate API and exchange keys with Vault.
- Use OPA admission control for Kubernetes policies.
- Enforce image signing (cosign) in CI/CD.
- Restrict egress and ingress with namespace network policies.
- Enable container runtime seccomp and read-only root filesystems.
