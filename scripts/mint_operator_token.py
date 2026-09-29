#!/usr/bin/env python3
"""Mint a short-lived HS256 operator JWT for the ΩMEGA PRIME Δ control plane.

Usage:
    JWT_SECRET=<secret> python3 scripts/mint_operator_token.py --role operator [--sub devon] [--hours 8] [--aud risk-service,ws-gateway]

Roles:
    operator — may activate the kill switch (POST /kill on risk-service)
    admin    — operator rights PLUS kill-switch reset (POST /reset)

Audiences (aud claim, enforced by each service):
    risk-service — accepted by risk-service /kill and /reset
    ws-gateway   — accepted by the websocket-gateway
A token is only accepted by the services named in its aud list. Mint the
narrowest audience the operator needs; never mint aud=risk-service for a
dashboard-only session.

Keep tokens short-lived (see docs/operator-token-policy.md) and never commit them.
"""

import argparse
import base64
import hashlib
import hmac
import json
import os
import sys
import time


def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


def mint(secret: str, sub: str, role: str, ttl_hours: float, aud: list) -> str:
    now = int(time.time())
    header = b64url(json.dumps({"alg": "HS256", "typ": "JWT"}, separators=(",", ":")).encode())
    payload = b64url(
        json.dumps(
            {
                "sub": sub,
                "role": role,
                "aud": aud,
                "exp": now + int(ttl_hours * 3600),
                "iat": now,
            },
            separators=(",", ":"),
        ).encode()
    )
    signing_input = f"{header}.{payload}".encode()
    sig = b64url(hmac.new(secret.encode(), signing_input, hashlib.sha256).digest())
    return f"{header}.{payload}.{sig}"


def main() -> int:
    ap = argparse.ArgumentParser(description="Mint an operator JWT for the control plane.")
    ap.add_argument("--role", required=True, choices=["operator", "admin"])
    ap.add_argument("--sub", default="operator")
    ap.add_argument("--hours", type=float, default=8, help="token lifetime in hours")
    ap.add_argument(
        "--aud",
        default="risk-service,ws-gateway",
        help="comma-separated audiences (default: risk-service,ws-gateway)",
    )
    args = ap.parse_args()

    secret = os.environ.get("JWT_SECRET", "").strip()
    if not secret:
        print("error: JWT_SECRET is not set", file=sys.stderr)
        return 1
    aud = [a.strip() for a in args.aud.split(",") if a.strip()]
    if not aud:
        print("error: --aud must name at least one audience", file=sys.stderr)
        return 1
    print(mint(secret, args.sub, args.role, args.hours, aud))
    return 0


if __name__ == "__main__":
    sys.exit(main())
