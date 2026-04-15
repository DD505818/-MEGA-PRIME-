RISK_LIMITS = {
    "risk_per_trade": 0.005,
    "daily_loss_limit": 0.02,
    "max_drawdown": 0.10,
    "max_exposure": 0.25,
}


def _coerce_metric(state, key):
    value = state.get(key, 0)
    try:
        return float(value)
    except (TypeError, ValueError) as exc:
        raise ValueError(f"state[{key!r}] must be numeric") from exc


def should_halt(state):
    if state is None:
        raise ValueError("state is required")
    if not isinstance(state, dict):
        raise TypeError("state must be a mapping")

    return any(
        [
            _coerce_metric(state, "loss_pct") >= RISK_LIMITS["daily_loss_limit"],
            _coerce_metric(state, "drawdown") >= RISK_LIMITS["max_drawdown"],
            _coerce_metric(state, "exposure") >= RISK_LIMITS["max_exposure"],
        ]
    )
