# Operator Token Policy

Enforced by `apps/risk-service/auth.go` and `apps/websocket-gateway/auth.js`.
Mint tokens with `scripts/mint_operator_token.py`.

## TTL policy

| Role     | Default TTL | Maximum TTL | Rationale |
|----------|-------------|-------------|-----------|
| operator | 8 hours     | 24 hours    | Covers an incident plus a shift handover. Short enough that a leaked token dies the same day. |
| admin    | 1 hour      | 4 hours     | Reset re-arms the system; it is the most dangerous action. Mint it for the incident at hand, not for the month. |

An operator token that expires in 15 minutes is useless during a 30-minute
incident; an admin token that lives for 30 days is a liability. The mint
script defaults (`--hours 8`) encode the table above. Override down for
routine drills, never up without a written reason.

## Audience policy

Tokens carry an `aud` claim and each service pins its own audience:

- `risk-service` — required by risk-service `/kill` and `/reset`.
- `ws-gateway` — required by the websocket-gateway.

A token is accepted only by the services named in its `aud` list, even though
both services share `JWT_SECRET`. Mint the narrowest audience the session
needs: a dashboard-only session gets `--aud ws-gateway` and cannot touch the
kill switch even if its role claim were tampered with (it can't be — the
signature covers the claims).

Distinct secrets per service remain the stronger option and are recommended
before live. Audience restriction is the floor, not the ceiling.

## Algorithm pinning

Both verifiers pin HS256 explicitly and never dispatch on the token header's
`alg` value:

- Go (`auth.go`): a single HMAC-SHA256 code path; the header is additionally
  required to say `alg=HS256`, and `typ` must be absent or `JWT`. There is no
  RSA code path, so the HMAC-secret-as-RSA-public-key confusion attack has
  nowhere to land.
- Node (`auth.js`): `jsonwebtoken` is called with `algorithms: ['HS256']`.

## Rejection cases (risk-service)

Every one of these is covered by `TestVerifyRejects` / `TestVerifyAcceptsAudVariants`:

1. Empty token
2. Malformed: not 3 segments
3. Malformed: invalid base64 in a segment
4. Tampered signature
5. Wrong secret
6. Expired `exp` (60s leeway)
7. `nbf` in the future (beyond 60s leeway)
8. `iat` in the future (beyond 60s leeway)
9. Unknown or missing `role`
10. Empty `sub`
11. `alg=none`
12. `alg` other than HS256 (incl. RS256 confusion attempt)
13. Unexpected `typ` (present and not `JWT`)
14. Missing `aud`, or `aud` not containing `risk-service`

Startup rejection (service refuses to boot): `JWT_SECRET` missing,
denylisted default (`change-me`, `dev-secret`, …), or shorter than 32 chars.

Transport rule: tokens are accepted **only** via `Authorization: Bearer`.
The old `?token=` query fallback was removed — credentials must not appear
in URLs (proxies, browsers, and servers log them).

## Planned before live (not in 1A)

- Key rotation: `kid` header + small keyring so a leaked `JWT_SECRET` can be
  rotated without downtime. Today, rotation revokes all outstanding tokens
  (documented in `docs/break-glass.md`).
- Rate limiting on `/kill` and `/reset` (per-IP token bucket).
- WS re-auth: long-lived connections currently authenticate once at
  handshake; cap connection lifetime to token TTL or re-auth on a timer.
