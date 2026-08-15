# ΩMEGA PRIME Δ — Deployment Package

**Release posture:** PAPER-first  
**Live trading:** disabled until broker certification, reconciliation, failure testing, and release-policy gates pass.

This document defines the reproducible handoff artifact produced by `scripts/build-deployment-package.sh`. The source repository remains canonical; the package is a categorized distribution of the verified implementation, not a second source tree.

## Package layout

```text
Omega-Prime-Deployment/
├── Core-Trading-Scripts/
├── Backend-Services/
├── Dashboard-UI/
├── Deployment-Tools/
├── PACKAGE_INFO.txt
└── SHA256SUMS
```

## 1. Core Trading Scripts

The package includes the verified trading/research paths:

```text
Core-Trading-Scripts/
├── apps/
│   ├── agent-service/
│   ├── execution-service/
│   ├── feature-engine/
│   ├── fusion-engine/
│   ├── market-data-service/
│   └── risk-service/
├── services/
│   └── agents/
├── strategies/
└── backtests/
```

Responsibilities:

- market ingestion and normalization
- feature generation
- AI/agent recommendations
- strategy generation and evaluation
- AEGIS deterministic risk gating
- PAPER execution and execution-state handling
- market/venue interfaces already present in the source tree

The governing boundary remains:

```text
AI / Strategies → AEGIS → Execution → Reconciliation / Portfolio → TruthCore
```

No AI agent is granted broker authority by this package.

## 2. Backend Services

```text
Backend-Services/
├── apps/
│   ├── capital-allocator/
│   ├── llm-service/
│   ├── portfolio-service/
│   ├── truth-core/
│   └── websocket-gateway/
└── backend/
```

Responsibilities:

- backend/API service code present in the canonical repository
- capital allocation
- LLM orchestration
- portfolio state
- WebSocket transport
- durable audit / TruthCore

### Blockchain status

`services/agents/mev-hunter` is included under **Core Trading Scripts** because the verified implementation is a market/MEV signal component. The current repository does **not** establish a production blockchain execution boundary with authenticated RPC, wallet signing, transaction broadcast, treasury approval, and on-chain reconciliation.

Therefore this package does not claim production blockchain execution. A future wallet/treasury gateway must remain isolated from strategy agents and require its own deterministic approval, signing, reconciliation, and audit controls.

## 3. Dashboard & UI

```text
Dashboard-UI/
└── apps/
    └── web-ui/
```

`apps/web-ui` is the canonical React/Next.js trading interface packaged for monitoring, risk controls, portfolio state, execution visibility, agent supervision, and system health.

No broker private key, exchange secret, wallet signing secret, or production credential belongs in frontend code.

## 4. Deployment Tools

```text
Deployment-Tools/
├── docker-compose.source.yml
├── docker-compose.package.yml
├── .env.example
├── infra/
│   └── db-migrations/
├── infrastructure/
│   ├── kafka/
│   ├── kubernetes/
│   ├── monitoring/
│   └── terraform/
├── helm/
│   └── omega-prime-delta/
├── github-workflows/
└── scripts/
```

`docker-compose.package.yml` is generated from the canonical root Compose file with build contexts rewritten for the categorized package layout. The source Compose file is preserved unchanged as `docker-compose.source.yml`.

The package also carries:

- Kubernetes manifests
- Terraform
- monitoring configuration
- Kafka infrastructure configuration
- Helm chart
- GitHub Actions definitions
- deployment/verification scripts
- safe `.env.example`

## Build the package

From the repository root:

```bash
bash scripts/build-deployment-package.sh
```

Default output:

```text
dist/Omega-Prime-Deployment/
dist/omega-prime-deployment-<git-sha>.tar.gz
```

If `zip` is available, a ZIP archive is emitted as well.

The builder:

1. verifies all required source paths exist;
2. copies only the mapped source trees;
3. excludes local environments, caches, VCS metadata, private-key files, credentials files, and build output;
4. rewrites only the packaged Docker Compose copy so its build contexts resolve inside the categorized artifact;
5. scans the staged files for common high-risk secret formats;
6. records the source commit and PAPER-only release posture;
7. creates SHA-256 checksums;
8. emits compressed deployment artifacts.

## Run the categorized Docker package

From inside the generated package:

```bash
cd Deployment-Tools
cp .env.example .env
# Keep PAPER_MODE=true and replace only development-safe values.
docker compose --env-file .env -f docker-compose.package.yml config
docker compose --env-file .env -f docker-compose.package.yml up -d --build
```

The current root Compose stack defaults trading services to `PAPER_MODE=true`. Do not override that for this release.

## Release invariants

The deployment artifact must preserve all of these:

```text
PAPER_MODE=true
LIVE_TRADING_ENABLED=false where defined
No broker credential in source or artifact
No wallet/private signing key in source or artifact
No AI-to-execution bypass
No AEGIS bypass
No unverified blockchain execution claim
No audit/reconciliation bypass
```

The artifact is a delivery mechanism. It does not itself certify LIVE readiness.
