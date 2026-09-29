# Break-Glass Procedure: Lost Operator/Admin Credentials

Read this now, not at 03:00 during an incident.

## What you can still do without the admin token

The fail-safe direction needs only the **operator** role:

- `POST /kill` (operator or admin) still works with any valid operator token.
- If you have no token at all but hold `JWT_SECRET`, mint one:
  `JWT_SECRET=<secret> python3 scripts/mint_operator_token.py --role operator --aud risk-service`

You do not need the admin token to stop the bleeding. You need it only to
re-arm (`POST /reset`).

## Admin token lost, JWT_SECRET known

The secret holder (Devon) mints a fresh admin token out of band:

```
JWT_SECRET=<secret> python3 scripts/mint_operator_token.py --role admin --sub devon-breakglass --hours 1 --aud risk-service
```

Deliver it to the incident commander over an existing trusted channel.
The old lost token remains valid until its `exp` — there is no revocation
list yet (see `docs/operator-token-policy.md`, "Planned before live"). Mint
with the shortest TTL that covers the incident (default 1h for admin).

## JWT_SECRET lost or suspected compromised

1. Generate a new secret: `python3 -c "import secrets; print(secrets.token_hex(32))"`
   (64 hex chars ≥ 32 chars; satisfies the startup check).
2. Update `JWT_SECRET` in the deployment (compose env / vault).
3. Restart risk-service and websocket-gateway.
   - Safe: the kill flag is durable in Redis (`kill_switch` key) and is
     restored on boot; a restart never clears a kill.
   - If Redis was unreachable at boot the service boots KILLED (fail closed)
     and retries — see `restoreControlState` in `apps/risk-service/main.go`.
4. Re-mint operator and admin tokens with the new secret.
5. Treat every token minted under the old secret as compromised: rotation
   revokes them all (no `kid` yet — rotation is atomic, not gradual).

## Redis lost (durable kill state unavailable)

- Risk-service boots KILLED and stays KILLED until it can read the durable
  flag. This is the fail-closed design; do not "fix" it by clearing anything.
- Restore Redis from backup/snapshot. The `kill_switch` key and the
  `control:audit` / `control:audit:gaps` streams are the state that matters.
- If the audit streams are lost, the stdout `CONTROL_AUDIT` / `CONTROL_AUDIT_GAP`
  log lines (with `audit_id`) are the fallback record.

## Contacts and custody

- `JWT_SECRET` custodian: Devon.
- Token minting is a human action via `scripts/mint_operator_token.py`; no
  service mints its own control-plane tokens.
- After any break-glass use, record: who minted, for whom, TTL, and why, in
  the incident log. The `sub` claim should identify the human (`--sub`).
