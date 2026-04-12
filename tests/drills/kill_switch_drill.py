import argparse

import requests


def drill_kill_switch(base_url: str, dry_run: bool = False) -> None:
    if dry_run:
        print("[DRY-RUN] kill switch activate -> reject order -> reset")
        return

    resp = requests.post(f"{base_url}/api/v1/kill", json={"reason": "drill"}, timeout=10)
    resp.raise_for_status()

    order_resp = requests.post(
        f"{base_url}/api/v1/orders",
        json={"symbol": "BTCUSD", "side": "buy", "qty": 0.1},
        timeout=10,
    )
    payload = order_resp.json()
    assert payload.get("status") == "kill_active", payload

    reset_resp = requests.post(f"{base_url}/api/v1/kill/reset", timeout=10)
    reset_resp.raise_for_status()


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://localhost:8000")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()
    drill_kill_switch(base_url=args.base_url, dry_run=args.dry_run)
