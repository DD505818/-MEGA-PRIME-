#!/usr/bin/env python3
import argparse
import sys

import requests


def validate(topic: str, registry_url: str) -> bool:
    artifact = f"{topic}-value"
    url = f"{registry_url}/groups/default/artifacts/{artifact}/versions/latest"
    response = requests.get(url, timeout=10)
    if response.status_code == 404:
        print(f"[WARN] No schema registered for {artifact}")
        return True
    response.raise_for_status()
    payload = response.json()
    if not payload.get("content"):
        print(f"[FAIL] Empty schema for {artifact}")
        return False
    print(f"[OK] Found schema for {artifact}")
    return True


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate schema registry artifacts")
    parser.add_argument("--topic", required=True)
    parser.add_argument("--registry-url", default="http://localhost:8081/apis/registry/v2")
    args = parser.parse_args()

    return 0 if validate(args.topic, args.registry_url) else 1


if __name__ == "__main__":
    sys.exit(main())
