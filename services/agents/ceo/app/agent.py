import time
from typing import Dict, Any

_active_agents: set = {
    "mamba","surge","arb","opt","gold","guard","maker","twin",
    "nexus","senti","orbit","oracle","harvest","stake","mev-hunter","midas","hi-darts"
}
_budgets: Dict[str, float] = {}

def get_status() -> Dict[str, Any]:
    return {"active_agents": sorted(_active_agents), "budget": _budgets, "ts": time.time()}

def activate(agent: str) -> Dict[str, Any]:
    _active_agents.add(agent)
    return {"action": "activated", "agent": agent}

def deactivate(agent: str) -> Dict[str, Any]:
    _active_agents.discard(agent)
    return {"action": "deactivated", "agent": agent}

def transform(payload: Dict[str, Any]) -> Dict[str, Any]:
    """Receive budget from CFO, relay commands to active agents."""
    _budgets.update(payload.get("allocations", {}))
    return {"event": "budget_applied", "agents": sorted(_active_agents), "ts": time.time()}
