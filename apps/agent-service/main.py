import asyncio

from health import HealthState, start_health_server
from modelock import require_mode
from orchestrator import Orchestrator
from strategies.box_theory import BoxTheory
from strategies.surge import Surge

# PAPER/LIVE contract: ambiguous or uncertified modes fail closed.
require_mode("agent-service")

health = HealthState()
start_health_server(health)
strategies = [BoxTheory(), Surge()]
orchestrator = Orchestrator(strategies, health)

if __name__ == "__main__":
    asyncio.run(orchestrator.run())
