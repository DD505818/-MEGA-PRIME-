"""Multi-region Redis client wrapper for active-active deployments."""

from __future__ import annotations

import os
import time
from concurrent.futures import ThreadPoolExecutor
from typing import Any

import redis


class ActiveActiveRedis:
    def __init__(self, region_configs: dict[str, str]) -> None:
        self.clients = {
            region: redis.Redis.from_url(url, decode_responses=True)
            for region, url in region_configs.items()
        }
        self.local_region = os.getenv("REGION", "us-east")
        self._writer_pool = ThreadPoolExecutor(max_workers=max(len(self.clients), 1))

    def set(self, key: str, value: Any, region_hint: str | None = None) -> None:
        payload = {"value": value, "timestamp": time.time()}
        target_regions = [region_hint] if region_hint else list(self.clients.keys())

        futures = [
            self._writer_pool.submit(self.clients[region].json().set, key, ".", payload)
            for region in target_regions
            if region in self.clients
        ]
        for future in futures:
            future.result()

    def get(self, key: str, region: str | None = None) -> Any:
        client = self.clients.get(region or self.local_region)
        if client is None:
            raise KeyError(f"unknown region: {region or self.local_region}")
        result = client.json().get(key)
        return result["value"] if result else None

    def get_with_failover(self, key: str) -> Any:
        preferred = [self.local_region] + [r for r in self.clients.keys() if r != self.local_region]
        for region in preferred:
            try:
                return self.get(key, region=region)
            except redis.ConnectionError:
                continue
        raise RuntimeError("all configured Redis regions are unreachable")
