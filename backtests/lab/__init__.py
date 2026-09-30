"""Leakage-safe validation primitives for ΩMEGA research.

Compatibility shim package: the canonical implementations live in the
``validation_lab`` package (``apps/validation-lab/validation_lab/``).
Modules under ``backtests.lab`` re-export them so existing imports and the
documented ``python -m backtests.lab.cli`` commands keep working. Do not
add new logic here; add it to ``validation_lab`` instead.
"""

import sys
from pathlib import Path

# Make the canonical package importable when running from the repo root
# (e.g. `python -m pytest backtests/tests`, `python -m backtests.lab.cli`)
# without requiring an installed distribution.
_VALIDATION_LAB_ROOT = Path(__file__).resolve().parents[2] / "apps" / "validation-lab"
if _VALIDATION_LAB_ROOT.is_dir() and str(_VALIDATION_LAB_ROOT) not in sys.path:
    sys.path.insert(0, str(_VALIDATION_LAB_ROOT))
