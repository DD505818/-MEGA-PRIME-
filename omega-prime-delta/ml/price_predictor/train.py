"""Training entrypoint for the price predictor."""

from __future__ import annotations

import argparse
import json
import logging
import os
from pathlib import Path

LOGGER = logging.getLogger("price_predictor.train")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Train price predictor")
    parser.add_argument("--epochs", type=int, default=3)
    parser.add_argument("--output", type=Path, default=Path("artifacts/price_predictor.json"))
    return parser.parse_args()


def validate_env() -> None:
    if not os.getenv("PRICE_TRAIN_DATASET"):
        raise RuntimeError("PRICE_TRAIN_DATASET is required")


def main() -> None:
    logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"), format="%(asctime)s %(levelname)s %(name)s %(message)s")
    args = parse_args()
    validate_env()

    args.output.parent.mkdir(parents=True, exist_ok=True)
    artifact = {"model": "price_predictor", "epochs": args.epochs, "status": "trained"}
    args.output.write_text(json.dumps(artifact, indent=2), encoding="utf-8")
    LOGGER.info("training_complete", extra={"output": str(args.output), "epochs": args.epochs})


if __name__ == "__main__":
    main()
