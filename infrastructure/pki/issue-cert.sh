#!/usr/bin/env bash
# ΩMEGA PRIME Δ — Per-service certificate issuance.
#
# Signs a server certificate for one service with the root CA created by
# make-ca.sh. RSA-2048, SHA-384, 2-year validity, SANs for the compose DNS
# name, localhost, and 127.0.0.1.
#
# Usage: issue-cert.sh <service> [--ca DIR] [--out DIR] [--days DAYS]
#   Defaults: ca=infrastructure/pki/ca, out=infrastructure/pki/certs/<service>
#
# Example: issue-cert.sh risk-engine
set -euo pipefail

usage() {
    cat <<'EOF'
Usage: issue-cert.sh <service> [--ca DIR] [--out DIR] [--days DAYS]

Issue a server certificate for <service>, signed by the ΩMEGA PRIME Δ root CA.

Arguments:
  service        Compose/DNS service name (e.g. risk-engine, kafka, postgres)

Options:
  --ca DIR   CA directory (default: infrastructure/pki/ca)
  --out DIR  Output directory (default: infrastructure/pki/certs/<service>)
  --days N   Validity in days (default: 730)
  -h, --help Show this help
EOF
}

SERVICE="${1:-}"
case "$SERVICE" in
    ""|-h|--help) usage; exit 0 ;;
esac
shift || true

CA_DIR="infrastructure/pki/ca"
OUT_DIR="infrastructure/pki/certs/${SERVICE}"
DAYS="730"

while [ $# -gt 0 ]; do
    case "$1" in
        --ca)  CA_DIR="$2"; shift 2 ;;
        --out) OUT_DIR="$2"; shift 2 ;;
        --days) DAYS="$2"; shift 2 ;;
        -h|--help) usage; exit 0 ;;
        *) echo "unknown option: $1" >&2; usage >&2; exit 2 ;;
    esac
done

command -v openssl >/dev/null 2>&1 || { echo "openssl not found" >&2; exit 1; }
[ -f "$CA_DIR/ca.key" ] && [ -f "$CA_DIR/ca.crt" ] \
    || { echo "CA not found in $CA_DIR — run make-ca.sh first." >&2; exit 1; }

mkdir -p "$OUT_DIR"
SAN="subjectAltName=DNS:${SERVICE},DNS:localhost,IP:127.0.0.1"

openssl genrsa -out "$OUT_DIR/${SERVICE}.key" 2048 2>/dev/null
chmod 600 "$OUT_DIR/${SERVICE}.key"
openssl req -new -key "$OUT_DIR/${SERVICE}.key" \
    -subj "/CN=${SERVICE}/O=Omega Prime" \
    -out "$OUT_DIR/${SERVICE}.csr" 2>/dev/null
openssl x509 -req -in "$OUT_DIR/${SERVICE}.csr" \
    -CA "$CA_DIR/ca.crt" -CAkey "$CA_DIR/ca.key" -CAcreateserial \
    -days "$DAYS" -sha384 -extfile <(printf '%s\n' "$SAN") \
    -out "$OUT_DIR/${SERVICE}.crt" 2>/dev/null
chmod 644 "$OUT_DIR/${SERVICE}.crt"
rm -f "$OUT_DIR/${SERVICE}.csr"

echo "Certificate issued for ${SERVICE}:"
echo "  key:  $OUT_DIR/${SERVICE}.key  (600 — deploy to the service only)"
echo "  cert: $OUT_DIR/${SERVICE}.crt  (644)"
openssl x509 -in "$OUT_DIR/${SERVICE}.crt" -noout -subject -dates -ext subjectAltName 2>/dev/null || true
