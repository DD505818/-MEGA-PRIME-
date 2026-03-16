from dataclasses import dataclass

@dataclass
class RiskLimits:
    risk_per_trade: float = 0.005
    daily_loss_limit: float = 0.02
    max_drawdown: float = 0.10
    max_exposure: float = 0.25


def should_halt(daily_loss: float, drawdown: float, exposure: float, limits: RiskLimits = RiskLimits()) -> bool:
    return (
        daily_loss >= limits.daily_loss_limit
        or drawdown >= limits.max_drawdown
        or exposure >= limits.max_exposure
    )
