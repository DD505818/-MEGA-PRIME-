#!/usr/bin/env bash
set -euo pipefail

DIRECTION="${1:-up}"

if [[ "$DIRECTION" != "up" && "$DIRECTION" != "down" ]]; then
  echo "usage: $0 [up|down]" >&2
  exit 1
fi

# Hook for migrate tool/container in production.
echo "Running schema migration: $DIRECTION"
