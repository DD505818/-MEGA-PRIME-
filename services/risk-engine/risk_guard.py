"""Backward-compatible risk guard helpers for tests and scripts."""

from dataclasses import dataclass


@dataclass(frozen=True)
class RiskLimits:
    risk_per_trade: float = 0.005
    daily_loss_limit: float = 0.02
    max_drawdown: float = 0.10
    max_exposure: float = 0.25


DEFAULT_LIMITS = RiskLimits()


def should_halt(
    daily_loss: float,
    drawdown: float,
    exposure: float,
    limits: RiskLimits | None = None,
) -> bool:
    active_limits = limits or DEFAULT_LIMITS
    return (
        daily_loss >= active_limits.daily_loss_limit
        or drawdown >= active_limits.max_drawdown
        or exposure >= active_limits.max_exposure
    )
