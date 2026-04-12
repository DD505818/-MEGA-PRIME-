RISK_LIMITS = {
    "risk_per_trade": 0.005,
    "daily_loss_limit": 0.02,
    "max_drawdown": 0.10,
    "max_exposure": 0.25,
}

def should_halt(state):
    if state is None:
        raise ValueError("state is required")

    return any([
        float(state.get("loss_pct", 0)) >= RISK_LIMITS["daily_loss_limit"],
        float(state.get("drawdown", 0)) >= RISK_LIMITS["max_drawdown"],
        float(state.get("exposure", 0)) >= RISK_LIMITS["max_exposure"],
    ])
