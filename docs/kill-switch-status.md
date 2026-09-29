# Kill Switch — Enforcement Status: ENFORCED

The kill switch **sets** a durable flag correctly (Phase 1A) and — as of
Phase 1B.2 — **enforces** it on every path that can submit an order.

## Who sets the flag

- `POST /kill` on risk-service (role: operator or admin) → `activateKillSwitch`
  → durable `kill_switch=1` in Redis (written before the in-memory flag), then
  `emergency.halt` published to Kafka.
- Internal triggers: Gate 7 (max drawdown), the cascade L3 path, and the 1B.2
  position reconciler call the same function under the same control-plane
  mutex.

Properties (tested): idempotent (second kill → 200 `already_active`),
serialized against `/reset` by `controlMu` with a monotonic `control:seq`,
durable-first commit order (a failed Redis write returns 503 and never sets
the in-memory flag), fail-closed boot (Redis down → assume KILLED until the
durable state is read).

## Who reads the flag today

| Reader | Behavior |
|--------|----------|
| risk-service Gate 1 (`validate`) | Reads the in-memory flag on every signal. New validations stop immediately after a kill. ✅ |
| execution-service `handleHalt` | Reacts to `emergency.halt` by cancelling routed / partially-filled / approved orders and writing `kill:confirmed`. Reactive. ✅ |
| execution-service `processSignal` | **Pre-submit check (1B.2):** reads the durable `kill_switch` flag before routing/filling. On `1`, the order transitions to CANCELLED with `cancel_reason=KILL_SWITCH_ACTIVE_AT_SUBMIT` and is never submitted. This closes the approve→submit race window. ✅ |

## Residual notes

- The pre-submit check reads the durable Redis flag rather than relying on
  the `emergency.halt` Kafka message, so it holds even if Kafka delivery lags
  or the execution-service restarted after the halt broadcast.
- No other service submits orders. The UI has no execution path (1A/1B UI
  lock: it may observe only).

## Related

- Enforcement code: `apps/risk-service/main.go` (`killHandler`, `resetHandler`,
  `restoreControlState`), `apps/risk-service/risk_engine.go`
  (`activateKillSwitch`, `resetKillSwitch`),
  `apps/execution-service/main.go` (`processSignal` pre-submit check,
  `handleHalt`).
- Token policy: `docs/operator-token-policy.md`.
- Break-glass: `docs/break-glass.md`.
