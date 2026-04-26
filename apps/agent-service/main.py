import asyncio
from orchestrator import Orchestrator
from strategies.box_theory import BoxTheory
from strategies.surge import Surge

strategies = [BoxTheory(), Surge()]
orchestrator = Orchestrator(strategies)

if __name__ == "__main__":
    asyncio.run(orchestrator.run())
