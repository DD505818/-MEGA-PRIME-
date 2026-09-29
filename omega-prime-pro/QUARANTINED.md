# ⛔ QUARANTINED — DO NOT USE

This directory is a **legacy parallel implementation** of ΩMEGA PRIME Δ.
It is frozen and quarantined as of Phase 1A (2026-09-28).

- **Canonical tree:** root `apps/` (backend) + `apps/web-ui` (frontend).
  See `docs/canonical-tree.md`.
- This tree is **not** built by root `docker-compose.yml`, **not** published
  by CI (`publish-ghcr.yml`), and **not** covered by the active test path.
- **Do not apply fixes here.** Security and behavior fixes land in the
  canonical tree only. Patches applied here will not propagate and will be
  discarded.
- **Do not import from here.** No canonical service may depend on this tree.
- Deletion is pending a diff/coverage analysis confirming nothing unique is
  needed. Until then: read-only history.

If you found this directory while looking for the real platform, go to the
repository root and read `README.md` and `docs/canonical-tree.md`.
