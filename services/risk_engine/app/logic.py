RISK_LIMITS = {
    "risk_per_trade": 0.005,
    "daily_loss_limit": 0.02,
    "max_drawdown": 0.10,
    "max_exposure": 0.25,
}

def should_halt(state):
    return any([
        state.get("loss_pct", 0) >= RISK_LIMITS["daily_loss_limit"],
        state.get("drawdown", 0) >= RISK_LIMITS["max_drawdown"],
        state.get("exposure", 0) >= RISK_LIMITS["max_exposure"],
    ])
