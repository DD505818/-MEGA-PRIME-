import time, math
from typing import Dict, Any

RISK_PER_TRADE  = 0.005   # 0.5%
DAILY_RISK_CAP  = 0.02    # 2.0%
MAX_DRAWDOWN    = 0.10    # 10%
TOTAL_CAPITAL   = 10_000.0

_daily_used  = 0.0
_equity_peak = TOTAL_CAPITAL

def get_budget() -> Dict[str, Any]:
    return {
        "risk_per_trade": RISK_PER_TRADE,
        "daily_cap": DAILY_RISK_CAP,
        "daily_used": _daily_used,
        "remaining": max(0.0, DAILY_RISK_CAP - _daily_used),
        "equity_peak": _equity_peak,
        "ts": time.time(),
    }

def allocate(agents: list) -> Dict[str, Any]:
    """Equal-weight risk allocation across active agents."""
    n = max(len(agents), 1)
    per_agent = RISK_PER_TRADE / n
    return {"allocations": {a: per_agent for a in agents}, "ts": time.time()}

def transform(payload: Dict[str, Any]) -> Dict[str, Any]:
    global _daily_used
    loss = payload.get("loss", 0.0)
    _daily_used = min(_daily_used + abs(loss), DAILY_RISK_CAP)
    var_95 = TOTAL_CAPITAL * RISK_PER_TRADE * math.sqrt(5) * 1.65
    return {
        "var_95": round(var_95, 2),
        "daily_used": _daily_used,
        "remaining": max(0.0, DAILY_RISK_CAP - _daily_used),
        "ts": time.time(),
    }
