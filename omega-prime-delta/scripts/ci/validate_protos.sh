#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROTO_DIR="$ROOT_DIR/contracts/proto"

if [[ ! -d "$PROTO_DIR" ]]; then
  echo "[proto-gate] ERROR: missing directory $PROTO_DIR"
  exit 1
fi

shopt -s globstar nullglob
proto_files=("$PROTO_DIR"/**/*.proto)
shopt -u globstar nullglob

if [[ ${#proto_files[@]} -eq 0 ]]; then
  echo "[proto-gate] ERROR: no .proto files found under $PROTO_DIR"
  exit 1
fi

for proto in "${proto_files[@]}"; do
  grep -q '^syntax = "proto3";' "$proto" || { echo "[proto-gate] ERROR: missing proto3 syntax in $proto"; exit 1; }
  grep -q '^package ' "$proto" || { echo "[proto-gate] ERROR: missing package declaration in $proto"; exit 1; }
  grep -q '^option go_package ' "$proto" || { echo "[proto-gate] ERROR: missing go_package option in $proto"; exit 1; }
  echo "[proto-gate] validated $proto"
done
