import asyncio

from health import HealthState, start_health_server
from modelock import require_paper
from orchestrator import Orchestrator
from strategies.box_theory import BoxTheory
from strategies.surge import Surge

# PAPER/LIVE lock: fail closed unless explicitly in paper mode.
require_paper("agent-service")

health = HealthState()
start_health_server(health)
strategies = [BoxTheory(), Surge()]
orchestrator = Orchestrator(strategies, health)

if __name__ == "__main__":
    asyncio.run(orchestrator.run())
