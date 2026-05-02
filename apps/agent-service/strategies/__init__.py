from .box_theory import BoxTheory
from .surge import Surge
from .arb import ARB
from .gap import GAP
from .rev import REV
from .senti import SENTI
from .twin import TWIN
from .maker import MAKER
from .harvest import HARVEST
from .gold import GOLD
from .opt import OPT
from .nexus import NEXUS

# Scaffolded agents — interface-ready, generate_signal returns None
from .guard import GUARD
from .aurum import AURUM
from .theta import THETA
from .phantom import PHANTOM
from .orbit import ORBIT
from .oracle import ORACLE
from .stake import STAKE
from .meme import MEME

# 12 production agents with real logic
PRODUCTION_AGENTS = [
    BoxTheory,
    Surge,
    ARB,
    GAP,
    REV,
    SENTI,
    TWIN,
    MAKER,
    HARVEST,
    GOLD,
    OPT,
    NEXUS,
]

# 7 scaffolded agents (return None; interface-ready for future integration)
SCAFFOLDED_AGENTS = [
    GUARD,
    AURUM,
    THETA,
    PHANTOM,
    ORBIT,
    ORACLE,
    STAKE,
    MEME,
]

ALL_AGENTS = PRODUCTION_AGENTS + SCAFFOLDED_AGENTS


def create_all_agents():
    """Instantiate all 19 agents (12 production + 7 scaffolded)."""
    return [cls() for cls in ALL_AGENTS]


def create_active_agents():
    """Instantiate only production agents with real signal logic."""
    return [cls() for cls in PRODUCTION_AGENTS]
