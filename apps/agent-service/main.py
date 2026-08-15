import asyncio

from health import HealthState, start_health_server
from orchestrator import Orchestrator
from strategies.box_theory import BoxTheory
from strategies.surge import Surge

health = HealthState()
start_health_server(health)
strategies = [BoxTheory(), Surge()]
orchestrator = Orchestrator(strategies, health)

if __name__ == "__main__":
    asyncio.run(orchestrator.run())
