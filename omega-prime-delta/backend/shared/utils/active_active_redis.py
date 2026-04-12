"""Multi-region active-active Redis helper.

Supports redis-py JSON API when present and degrades to basic string values.
"""

from __future__ import annotations

import json
import os
import time
from typing import Any, Dict, Optional

import redis


class ActiveActiveRedis:
    def __init__(self, region_configs: Dict[str, str], local_region: Optional[str] = None):
        if not region_configs:
            raise ValueError("region_configs must not be empty")

        self.clients = {
            region: redis.Redis.from_url(url, decode_responses=True)
            for region, url in region_configs.items()
        }
        self.local_region = local_region or os.getenv("REGION", "us-east")

    def set(self, key: str, value: Any, region_hint: Optional[str] = None) -> None:
        payload = {"value": value, "timestamp": time.time()}
        target_regions = [region_hint] if region_hint else list(self.clients.keys())
        for region in target_regions:
            client = self.clients[region]
            try:
                client.json().set(key, ".", payload)
            except Exception:
                client.set(key, json.dumps(payload))

    def get(self, key: str, region: Optional[str] = None) -> Any:
        target_region = region or self.local_region
        client = self.clients[target_region]
        try:
            result = client.json().get(key)
            return result["value"] if result else None
        except Exception:
            raw = client.get(key)
            if not raw:
                return None
            parsed = json.loads(raw)
            return parsed.get("value")

    def resilient_get(self, key: str, fallback_regions: Optional[list[str]] = None) -> Any:
        candidates = [self.local_region] + (fallback_regions or [])
        seen = set()
        for region in candidates:
            if region in seen or region not in self.clients:
                continue
            seen.add(region)
            try:
                value = self.get(key, region=region)
                if value is not None:
                    return value
            except redis.ConnectionError:
                continue
        raise RuntimeError("all Redis regions unreachable or key not found")
