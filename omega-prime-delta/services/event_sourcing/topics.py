"""Topic constants for ΩMEGA Prime event sourcing."""

TRADES_TOPIC = "omega.trades"
RISK_TOPIC = "omega.risk"

SUPPORTED_EVENT_TYPES = {
    "ORDER_PLACED",
    "ORDER_FILLED",
    "ORDER_REJECTED",
    "RISK_CHECK",
}
