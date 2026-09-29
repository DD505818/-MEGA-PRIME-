# Canonical Tree Declaration

**Effective:** 2026-09-28 (Phase 1A)

## Canonical (active) tree

The platform that is built, deployed, tested, and hardened is:

- **Backend:** `apps/` — the 12 services wired by root `docker-compose.yml`
  (market-data-service, feature-engine, agent-service, fusion-engine,
  risk-service, execution-service, capital-allocator, portfolio-service,
  truth-core, llm-service, websocket-gateway, strategy-engine)
- **Frontend:** `apps/web-ui` (Next.js)
- **Contracts:** root `schemas/`, service docs under `docs/`
- **Infra entry points:** root `docker-compose.yml`, `Makefile`, `infra/`

Evidence: root `docker-compose.yml` builds `apps/*` + `apps/web-ui` only;
`publish-ghcr.yml` publishes the 12 `apps/*` contexts + `web-ui`;
`paper-chain.yml` exercises the root compose stack.

## Quarantined (legacy) trees — do not build, deploy, or patch

| Tree | Status |
|---|---|
| `omega-prime-delta/` | QUARANTINED — parallel Go backend + frontend prototype |
| `omega-prime-hardened/` | QUARANTINED — byte-copy of delta with weaker risk logic |
| `omega-prime-pro/` | QUARANTINED — Python stub services, print-and-exit |

Each contains a `QUARANTINED.md` marker. Rules:

1. **No security or behavior fixes are applied to quarantined trees.** Fixes land
   in the canonical tree only; the quarantined copies are frozen.
2. **CI must not treat them as active.** The `canonical-tree-guard` workflow
   fails if root compose, Makefile, or publish contexts reference them.
3. **Deletion is pending** a diff/coverage analysis confirming nothing unique is
   needed from them. Do not delete until that analysis is reviewed.

## Known non-canonical leftovers (not quarantined, pending decision)

- `services/` + `gateway-api/` (Python): still exercised by `ci-cd.yml`'s test
  stage and build matrix. This is a pipeline/compose mismatch to resolve in a
  later phase — the compose-deployed authority path is `apps/`.
- `dashboard-react/`: still built by CI and referenced by K8s manifests; not the
  canonical frontend (`apps/web-ui` is). Pending migration/removal decision.
- `control-panel/`, `dashboard/`: Express stubs. Not deployed by root compose.

## Policy for contributors

If you are about to edit code outside `apps/`, `docs/`, `infra/`, `scripts/`,
or root config: stop. Check this file first. New work goes in the canonical
tree; quarantined trees are read-only history.
