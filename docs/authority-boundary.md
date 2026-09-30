# Authority Boundary: Signed Single-Use AEGIS Approvals

**Status:** implemented on `phase/3-authority-boundary` (base `87b3301`).
**Governing state:** EDGE NOT PROVEN · LIVE LOCKED · PAPER-first.

## Problem

`risk-service` (AEGIS) used to forward the *same mutable signal map* to
`signals.approved` with `risk_approved=true`, and `execution-service`
(VULTURE) trusted it implicitly:

```go
// Risk approval already happened (signal came from signals.approved)
```

Anyone able to produce to the `signals.approved` Kafka topic could inject
an order: no signature, no replay protection, no field binding, and the
kill-switch pre-submit check failed open on Redis errors (`.Val()`).

## Design

Every order VULTURE may submit must carry a **signed, single-use, expiring
approval** issued by AEGIS. The approval is the *only* execution authority;
trust in the topic is not enough.

### The approval (`apps/approval/`)

An `Approval` binds every execution-critical field:

- `approval_id` (uuid, the single-use nonce)
- `signal_id`, `strategy_id`, `symbol`, `side`
- `quantity` (the **risk-adjusted** value), `limit_price`, `stop_price`, `mode`
- `issued_at_ms`, `expires_at_ms` (60s TTL), `gates_version`

The Ed25519 signature covers canonical JSON bytes of all fields above
(fixed field order; the signature itself is excluded). Verification is
independent at VULTURE.

### Issuance (risk-service)

After the 14 gates approve, `run()` builds the approval over the adjusted
quantity, signs it with `AEGIS_APPROVAL_PRIVKEY`, and attaches it as
`signal["aegis_approval"]`. **An approved-but-unsigned signal is never
forwarded**: missing key or signing failure routes to `signals.rejected`
with `APPROVAL_SIGNING_UNAVAILABLE` / `APPROVAL_SIGNING_FAILED` plus a
critical `risk.alerts` event.

The service refuses to start without a valid private key
(`authority boundary: ...` fatal at startup).

### Verification (execution-service)

`processSignal()` verifies **before any order state or routing**, in order:

1. `APPROVAL_MISSING` / `APPROVAL_MALFORMED` — no usable approval attached.
2. `APPROVAL_BAD_SIGNATURE` — Ed25519 verify against `AEGIS_APPROVAL_PUBKEY`.
3. `APPROVAL_EXPIRED` — past the 60s validity window.
4. `APPROVAL_FIELD_MISMATCH` — signal's execution-critical fields must equal
   the approved values exactly (catches post-approval tampering).
5. `APPROVAL_REPLAY` — atomic `SET aegis:approval:<id> claimed NX PX 10m`
   in Redis. The single-use guarantee is enforced by **Redis atomicity**
   (the database boundary), not by process memory.
6. `APPROVAL_CLAIM_STORE_UNAVAILABLE` — Redis error during the claim fails
   closed.

Refused orders are cancelled with the reason in `order.Meta`
(`cancel_reason`), never submitted. Verified orders carry
`approval_id` + `aegis_gates_version` in `Meta` for lineage.

The service refuses to start without a valid public key.

### Kill-switch pre-submit (fail-open fix)

The old `redis.Get(...).Val()` failed **open** on Redis errors (empty
string ≠ `"1"` → submit). It is now `killActiveAtSubmit()` with explicit
error handling: unreadable kill state is `KILL_SWITCH_STATE_UNKNOWN` and
refuses submission. The 60s approval TTL additionally bounds the
approve→submit race window.

## Key management

- Generate: `go run ./apps/approval/cmd/keygen` (from `apps/approval/`).
- `AEGIS_APPROVAL_PRIVKEY` (base64 32-byte seed) → risk-service env only.
- `AEGIS_APPROVAL_PUBKEY` (base64 32-byte) → execution-service env.
- Never commit keys, never log them. `docker-compose.yml` fails fast
  (`${VAR:?...}`) if either is unset.
- Rotation: generate a new pair, deploy the public key to execution first,
  then the private key to risk. Approvals are 60s-lived, so rotation is
  hitless. Key ID binding is reserved for the next iteration.

## Threat model (adversarial tests)

`apps/execution-service/authority_test.go` covers:

| Attack | Expected |
|---|---|
| Raw injection to `signals.approved` (no approval) | `APPROVAL_MISSING` |
| Field tamper after signing (qty, price, stop, side, symbol, signal_id, mode) | `APPROVAL_FIELD_MISMATCH` |
| Corrupted signature / unknown signing key | `APPROVAL_BAD_SIGNATURE` |
| Expired approval (valid signature) | `APPROVAL_EXPIRED` |
| Replay of a consumed approval (fresh Kafka message, same approval_id) | `APPROVAL_REPLAY` |
| Redis down at claim time | `APPROVAL_CLAIM_STORE_UNAVAILABLE` |
| Redis down at kill check | `KILL_SWITCH_STATE_UNKNOWN` (was: fail-open submit) |
| Kill active at submit | `KILL_SWITCH_ACTIVE_AT_SUBMIT` via `processSignal` |

`apps/approval/approval_test.go` covers sign/verify round-trip, per-field
tamper rejection, wrong-key rejection, expiry, canonical determinism, key
parsing, and the Kafka map round-trip. `apps/risk-service/authority_test.go`
covers issuance binding the adjusted quantity and surviving forwarding.

## What this does NOT do (next phases)

- TruthCore issuance/consumption records (`aegis.approval_issued`,
  `vulture.approval_consumed`) — Phase 4 (audit-or-no-trade).
- Key IDs / multi-key rotation without restart.
- Cross-checking the approval against an independent TruthCore read at
  submit time (latency tradeoff; the Redis claim is the enforcement point).
