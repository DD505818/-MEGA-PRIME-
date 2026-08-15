import asyncio
import json
import os
import pickle
from collections import deque

from aiokafka import AIOKafkaConsumer, AIOKafkaProducer
import numpy as np
import redis.asyncio as redis
import talib

from health import HealthState, start_health_server
from transport import kafka_client_kwargs


class FeatureEngine:
    def __init__(self, health: HealthState):
        kafka_kwargs = kafka_client_kwargs()
        self.consumer = AIOKafkaConsumer("market.raw", **kafka_kwargs)
        self.producer = AIOKafkaProducer(**kafka_kwargs)
        self.redis = redis.from_url(os.getenv("REDIS_URL", "redis://redis:6379"))
        self.buffer = {}
        self.health = health

    async def run(self):
        consumer_started = False
        producer_started = False
        try:
            await self.consumer.start()
            consumer_started = True
            await self.producer.start()
            producer_started = True
            await self.redis.ping()
            self.health.ready()
            async for msg in self.consumer:
                tick = json.loads(msg.value)
                symbol = tick["symbol"]
                price = tick["price"]
                key = f"prices:{symbol}"
                data = await self.redis.get(key)
                window = pickle.loads(data) if data else deque(maxlen=200)
                window.append(price)
                await self.redis.set(key, pickle.dumps(window))
                arr = np.array(window, dtype=np.float64)
                if len(arr) >= 14:
                    features = {
                        "symbol": symbol,
                        "rsi": talib.RSI(arr, timeperiod=14)[-1],
                        "sma": talib.SMA(arr, timeperiod=20)[-1],
                        "norm_price": (price - np.mean(arr)) / (np.std(arr) + 1e-7),
                    }
                    await self.producer.send(
                        "features.norm", json.dumps(features).encode()
                    )
        except Exception:
            self.health.not_ready("kafka_or_redis")
            raise
        finally:
            self.health.not_ready("shutdown")
            if consumer_started:
                await self.consumer.stop()
            if producer_started:
                await self.producer.stop()
            await self.redis.aclose()


if __name__ == "__main__":
    health = HealthState()
    start_health_server(health)
    asyncio.run(FeatureEngine(health).run())
