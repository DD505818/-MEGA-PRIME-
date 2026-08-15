#!/usr/bin/env bash
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
OUT_DIR="${1:-$ROOT/dist}"
PACKAGE_NAME="Omega-Prime-Deployment"
STAGE="$OUT_DIR/$PACKAGE_NAME"
SHORT_SHA="$(git -C "$ROOT" rev-parse --short=12 HEAD)"
SOURCE_SHA="$(git -C "$ROOT" rev-parse HEAD)"
HELM_SOURCE="omega-prime-delta/infrastructure/helm/omega-prime-delta"
CLOUDFLARE_EDGE_REPOSITORY="DD505818/omegaaa"
CLOUDFLARE_EDGE_COMMIT="a8fe606ec4f4f8539baf26321b521b80e1db5d1a"

CORE_PATHS=(
  "apps/agent-service"
  "apps/execution-service"
  "apps/feature-engine"
  "apps/fusion-engine"
  "apps/market-data-service"
  "apps/risk-service"
  "services/agents"
  "strategies"
  "backtests"
)

BACKEND_PATHS=(
  "apps/capital-allocator"
  "apps/llm-service"
  "apps/portfolio-service"
  "apps/truth-core"
  "apps/websocket-gateway"
  "backend"
)

DASHBOARD_PATHS=(
  "apps/web-ui"
)

DEPLOYMENT_PATHS=(
  "infra/db-migrations"
  "infrastructure/kafka"
  "infrastructure/kubernetes"
  "infrastructure/monitoring"
  "infrastructure/terraform"
  "scripts"
)

RSYNC_EXCLUDES=(
  "--exclude=.git"
  "--exclude=.env"
  "--exclude=.env.*"
  "--exclude=.npmrc"
  "--exclude=.pypirc"
  "--exclude=.netrc"
  "--exclude=node_modules"
  "--exclude=__pycache__"
  "--exclude=.pytest_cache"
  "--exclude=.mypy_cache"
  "--exclude=.ruff_cache"
  "--exclude=.next"
  "--exclude=dist"
  "--exclude=build"
  "--exclude=.cache"
  "--exclude=.venv"
  "--exclude=venv"
  "--exclude=*.pem"
  "--exclude=*.key"
  "--exclude=id_rsa"
  "--exclude=id_ed25519"
  "--exclude=credentials.json"
  "--exclude=credentials.yml"
  "--exclude=credentials.yaml"
)

require_path() {
  local path="$1"
  if [[ ! -e "$ROOT/$path" ]]; then
    echo "ERROR: required source path is missing: $path" >&2
    exit 1
  fi
}

copy_group() {
  local destination="$1"
  shift

  mkdir -p "$STAGE/$destination"
  for source_path in "$@"; do
    require_path "$source_path"
    (
      cd "$ROOT"
      rsync -aR "${RSYNC_EXCLUDES[@]}" "./$source_path" "$STAGE/$destination/"
    )
  done
}

scan_for_secrets() {
  local forbidden_files
  forbidden_files="$(
    find "$STAGE" -type f \
      \( -name '.env' \
      -o -name '*.pem' \
      -o -name '*.key' \
      -o -name 'id_rsa' \
      -o -name 'id_ed25519' \
      -o -name '.npmrc' \
      -o -name '.pypirc' \
      -o -name '.netrc' \
      -o -name 'credentials.json' \
      -o -name 'credentials.yml' \
      -o -name 'credentials.yaml' \) \
      -print
  )"

  if [[ -n "$forbidden_files" ]]; then
    echo "ERROR: forbidden credential/private-key files entered the deployment package:" >&2
    printf '%s\n' "$forbidden_files" >&2
    exit 1
  fi

  local secret_hits
  secret_hits="$(
    grep -RIlE \
      '(-----BEGIN [A-Z ]*PRIVATE KEY-----|sk_(live|test)_[A-Za-z0-9]{16,}|whsec_[A-Za-z0-9]{16,}|ghp_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16}|xox[baprs]-[A-Za-z0-9-]{16,})' \
      "$STAGE" 2>/dev/null || true
  )"

  if [[ -n "$secret_hits" ]]; then
    echo "ERROR: probable secret material detected in deployment package:" >&2
    printf '%s\n' "$secret_hits" >&2
    exit 1
  fi
}

rm -rf "$STAGE"
mkdir -p "$OUT_DIR" "$STAGE"

copy_group "Core-Trading-Scripts" "${CORE_PATHS[@]}"
copy_group "Backend-Services" "${BACKEND_PATHS[@]}"
copy_group "Dashboard-UI" "${DASHBOARD_PATHS[@]}"
copy_group "Deployment-Tools" "${DEPLOYMENT_PATHS[@]}"

# Flatten the legacy nested Helm source path into the deployment-facing layout.
require_path "$HELM_SOURCE"
mkdir -p "$STAGE/Deployment-Tools/helm/omega-prime-delta"
rsync -a "${RSYNC_EXCLUDES[@]}" "$ROOT/$HELM_SOURCE/" "$STAGE/Deployment-Tools/helm/omega-prime-delta/"

# Preserve current CI/CD definitions as deployment evidence/configuration.
mkdir -p "$STAGE/Deployment-Tools/github-workflows"
require_path ".github/workflows"
rsync -a "${RSYNC_EXCLUDES[@]}" "$ROOT/.github/workflows/" "$STAGE/Deployment-Tools/github-workflows/"

# Include only the checked-in safe environment template, never a local .env.
require_path ".env.example"
cp "$ROOT/.env.example" "$STAGE/Deployment-Tools/.env.example"

# Preserve the canonical Compose file and generate a categorized-package variant.
require_path "docker-compose.yml"
cp "$ROOT/docker-compose.yml" "$STAGE/Deployment-Tools/docker-compose.source.yml"

sed \
  -e 's#build: \./apps/market-data-service#build: ../Core-Trading-Scripts/apps/market-data-service#' \
  -e 's#build: \./apps/feature-engine#build: ../Core-Trading-Scripts/apps/feature-engine#' \
  -e 's#build: \./apps/agent-service#build: ../Core-Trading-Scripts/apps/agent-service#' \
  -e 's#build: \./apps/fusion-engine#build: ../Core-Trading-Scripts/apps/fusion-engine#' \
  -e 's#build: \./apps/risk-service#build: ../Core-Trading-Scripts/apps/risk-service#' \
  -e 's#build: \./apps/execution-service#build: ../Core-Trading-Scripts/apps/execution-service#' \
  -e 's#build: \./apps/capital-allocator#build: ../Backend-Services/apps/capital-allocator#' \
  -e 's#build: \./apps/portfolio-service#build: ../Backend-Services/apps/portfolio-service#' \
  -e 's#build: \./apps/truth-core#build: ../Backend-Services/apps/truth-core#' \
  -e 's#build: \./apps/llm-service#build: ../Backend-Services/apps/llm-service#' \
  -e 's#build: \./apps/websocket-gateway#build: ../Backend-Services/apps/websocket-gateway#' \
  -e 's#build: \./apps/web-ui#build: ../Dashboard-UI/apps/web-ui#' \
  "$ROOT/docker-compose.yml" > "$STAGE/Deployment-Tools/docker-compose.package.yml"

# Pin the separately deployed Cloudflare Worker instead of importing another repo at build time.
mkdir -p "$STAGE/Deployment-Tools/cloudflare"
cat > "$STAGE/Deployment-Tools/cloudflare/omegaaa-companion.txt" <<EOF
ΩMEGA PRIME Δ Cloudflare edge companion
repository=$CLOUDFLARE_EDGE_REPOSITORY
pinned_commit=$CLOUDFLARE_EDGE_COMMIT
role=public REST API + Workers AI inference edge
release_mode=PAPER
live_trading_enabled=false
execution_authority=false
required_secret=OMEGA_API_TOKEN
EOF

cp "$ROOT/DEPLOYMENT_PACKAGE.md" "$STAGE/README.md"

cat > "$STAGE/PACKAGE_INFO.txt" <<EOF
ΩMEGA PRIME Δ deployment package
source_repository=DD505818/-MEGA-PRIME-
source_commit=$SOURCE_SHA
cloudflare_edge_repository=$CLOUDFLARE_EDGE_REPOSITORY
cloudflare_edge_commit=$CLOUDFLARE_EDGE_COMMIT
release_mode=PAPER
paper_mode_required=true
live_trading_enabled=false
ai_execution_authority=false
blockchain_execution_certified=false
built_at_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF

scan_for_secrets

(
  cd "$STAGE"
  find . -type f ! -name 'SHA256SUMS' -print0 \
    | sort -z \
    | xargs -0 sha256sum > SHA256SUMS
)

TAR_PATH="$OUT_DIR/omega-prime-deployment-$SHORT_SHA.tar.gz"
rm -f "$TAR_PATH"
tar -C "$OUT_DIR" -czf "$TAR_PATH" "$PACKAGE_NAME"

echo "Created: $TAR_PATH"

if command -v zip >/dev/null 2>&1; then
  ZIP_PATH="$OUT_DIR/omega-prime-deployment-$SHORT_SHA.zip"
  rm -f "$ZIP_PATH"
  (
    cd "$OUT_DIR"
    zip -qr "$(basename "$ZIP_PATH")" "$PACKAGE_NAME"
  )
  echo "Created: $ZIP_PATH"
fi

echo "Deployment package build complete."
