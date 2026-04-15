import os

# Fail-closed defaults.
LIVE_EXECUTION_ENABLED = os.getenv("LIVE_EXECUTION_ENABLED", "false").lower() == "true"
PAPER_TRADING_ONLY = os.getenv("PAPER_TRADING_ONLY", "true").lower() == "true"
OPERATOR_APPROVAL_REQUIRED = os.getenv("OPERATOR_APPROVAL_REQUIRED", "true").lower() == "true"


def can_execute_live() -> bool:
    return LIVE_EXECUTION_ENABLED and not PAPER_TRADING_ONLY
