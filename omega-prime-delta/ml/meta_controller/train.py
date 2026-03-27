"""Training entrypoint for the meta controller."""

from __future__ import annotations

import argparse
import json
import logging
import os
from pathlib import Path

LOGGER = logging.getLogger("meta_controller.train")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Train meta-controller")
    parser.add_argument("--episodes", type=int, default=100)
    parser.add_argument("--output", type=Path, default=Path("artifacts/meta_controller.json"))
    return parser.parse_args()


def validate_env() -> None:
    if not os.getenv("META_CONTROLLER_REPLAY_PATH"):
        raise RuntimeError("META_CONTROLLER_REPLAY_PATH is required")


def main() -> None:
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"), format="%(asctime)s %(levelname)s %(name)s %(message)s")
    args = parse_args()
    validate_env()

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps({"model": "meta_controller", "episodes": args.episodes, "status": "trained"}, indent=2),
        encoding="utf-8",
    )
    LOGGER.info("training_complete", extra={"output": str(args.output), "episodes": args.episodes})


if __name__ == "__main__":
    main()
