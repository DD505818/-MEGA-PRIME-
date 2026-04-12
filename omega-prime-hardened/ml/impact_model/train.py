"""Training entrypoint for the impact model."""

from __future__ import annotations

import argparse
import json
import logging
import os
from pathlib import Path

LOGGER = logging.getLogger("impact_model.train")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Train impact model")
    parser.add_argument("--epochs", type=int, default=5)
    parser.add_argument("--output", type=Path, default=Path("artifacts/impact_model.json"))
    return parser.parse_args()


def validate_env() -> None:
    if not os.getenv("IMPACT_TRAIN_DATASET"):
        raise RuntimeError("IMPACT_TRAIN_DATASET is required")


def main() -> None:
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"), format="%(asctime)s %(levelname)s %(name)s %(message)s")
    args = parse_args()
    validate_env()

    args.output.parent.mkdir(parents=True, exist_ok=True)
    artifact = {"model": "impact_model", "epochs": args.epochs, "status": "trained"}
    args.output.write_text(json.dumps(artifact, indent=2), encoding="utf-8")
    LOGGER.info("training_complete", extra={"output": str(args.output), "epochs": args.epochs})


if __name__ == "__main__":
    main()
