#!/usr/bin/env python3
from __future__ import annotations

import pathlib
import sys

import yaml

ROOT = pathlib.Path(__file__).resolve().parents[2]
CONTRACTS_FILE = ROOT / "contracts" / "service-contracts.yaml"


def fail(msg: str) -> None:
    print(f"[contracts-gate] ERROR: {msg}")
    sys.exit(1)


def main() -> None:
    if not CONTRACTS_FILE.exists():
        fail(f"missing file: {CONTRACTS_FILE}")

    with CONTRACTS_FILE.open("r", encoding="utf-8") as fh:
        data = yaml.safe_load(fh)

    for key in ["version", "system", "http_contracts", "kafka_contracts", "slos"]:
        if key not in data:
            fail(f"required key '{key}' is missing")

    if not isinstance(data["http_contracts"], list) or not data["http_contracts"]:
        fail("http_contracts must be a non-empty list")

    if not isinstance(data["kafka_contracts"], list) or not data["kafka_contracts"]:
        fail("kafka_contracts must be a non-empty list")

    for idx, contract in enumerate(data["http_contracts"]):
        for key in ["service", "base_path", "endpoints"]:
            if key not in contract:
                fail(f"http_contracts[{idx}] missing '{key}'")

    for idx, contract in enumerate(data["kafka_contracts"]):
        for key in ["topic", "key", "value_schema", "producers", "consumers"]:
            if key not in contract:
                fail(f"kafka_contracts[{idx}] missing '{key}'")

    print("[contracts-gate] service contracts validated")


if __name__ == "__main__":
    main()
