import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
AGENT_SERVICE = ROOT / "apps" / "agent-service"

for path in (ROOT, AGENT_SERVICE):
    path_string = str(path)
    if path_string not in sys.path:
        sys.path.insert(0, path_string)
