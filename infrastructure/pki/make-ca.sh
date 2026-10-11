#!/usr/bin/env bash
# ΩMEGA PRIME Δ — Root CA generation.
#
# Operator-run, ideally on an offline machine. Creates a self-signed root CA
# (RSA-4096, SHA-384, 10-year validity) used to sign per-service certificates
# via infrastructure/pki/issue-cert.sh.
#
# Usage: make-ca.sh [--out DIR] [--cn COMMON_NAME] [--days DAYS]
#   Defaults: DIR=infrastructure/pki/ca, CN=OMEGA-PRIME-ROOT-CA, DAYS=3650
#
# The CA private key (ca.key) is the crown jewel: chmod 600, never committed,
# never copied to the deployment host. Only ca.crt ships with services.
set -euo pipefail

OUT_DIR="infrastructure/pki/ca"
CN="OMEGA-PRIME-ROOT-CA"
DAYS="3650"

usage() {
    cat <<'EOF'
Usage: make-ca.sh [--out DIR] [--cn COMMON_NAME] [--days DAYS]

Generate the ΩMEGA PRIME Δ root CA (self-signed, RSA-4096, SHA-384).

Options:
  --out DIR   Output directory (default: infrastructure/pki/ca)
  --cn NAME   CA common name (default: OMEGA-PRIME-ROOT-CA)
  --days N    Validity in days (default: 3650)
  -h, --help  Show this help
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        --out)  OUT_DIR="$2"; shift 2 ;;
        --cn)   CN="$2"; shift 2 ;;
        --days) DAYS="$2"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
    esac
done

command -v openssl >/dev/null 2>&1 || { echo "openssl not found" >&2; exit 1; }

mkdir -p "$OUT_DIR"
if [ -e "$OUT_DIR/ca.key" ]; then
    echo "REFUSING: $OUT_DIR/ca.key already exists — rotate per ROTATION.md, never overwrite." >&2
    exit 1
fi

openssl genrsa -out "$OUT_DIR/ca.key" 4096 2>/dev/null
chmod 600 "$OUT_DIR/ca.key"
openssl req -x509 -new -nodes \
    -key "$OUT_DIR/ca.key" -sha384 -days "$DAYS" \
    -subj "/CN=${CN}/O=Omega Prime" \
    -out "$OUT_DIR/ca.crt" 2>/dev/null
chmod 644 "$OUT_DIR/ca.crt"

echo "Root CA written:"
echo "  private: $OUT_DIR/ca.key  (600 — NEVER commit, NEVER deploy)"
echo "  public:  $OUT_DIR/ca.crt  (644 — distribute to services as trust anchor)"
openssl x509 -in "$OUT_DIR/ca.crt" -noout -subject -dates
