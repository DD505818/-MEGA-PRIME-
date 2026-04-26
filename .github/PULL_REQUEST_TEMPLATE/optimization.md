# Optimization / Risk-Hardening PR

Linked issue: #37

## Scope

- [ ] Signal quality / ALPHA-BETA-GAMMA-DELTA
- [ ] AEGIS risk hardening
- [ ] CAFÉ-RC fusion
- [ ] Execution edge
- [ ] Frontend operator controls
- [ ] Validation / backtest / chaos testing
- [ ] Documentation only

## Safety Invariants

- [ ] No order bypasses AEGIS Governor
- [ ] Kill state overrides new logic
- [ ] Daily loss limit overrides new logic
- [ ] Max drawdown limit overrides new logic
- [ ] Per-trade risk cap overrides new logic
- [ ] Paper/live state remains explicit
- [ ] No production API keys or secrets committed

## Validation Evidence

- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Backtest/walk-forward results attached, if performance is claimed
- [ ] Monte Carlo results attached, if sizing/leverage changed
- [ ] Chaos test evidence attached, if execution/risk behavior changed

## Operator Impact

Describe what an operator sees, what can fail, and what the system does when it fails.

## Rollback Plan

Describe how to disable this change by config or revert safely.
