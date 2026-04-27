from .box_theory import BoxTheory
from .surge import Surge
from .gap import GAP
from .rev import REV
from .senti import SENTI
from .twin import TWIN
from .maker import MAKER
from .harvest import HARVEST

ACTIVE_AGENTS = [
    BoxTheory,
    Surge,
    GAP,
    REV,
    SENTI,
    TWIN,
    MAKER,
    HARVEST,
]


def create_active_agents():
    """Instantiate only implemented, importable agents."""
    return [agent_cls() for agent_cls in ACTIVE_AGENTS]
