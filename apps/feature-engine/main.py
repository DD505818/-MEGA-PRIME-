import asyncio, json, pickle
from collections import deque
from aiokafka import AIOKafkaConsumer, AIOKafkaProducer
import numpy as np
import redis.asyncio as redis
import talib

class FeatureEngine:
    def __init__(self):
        self.consumer = AIOKafkaConsumer('market.raw', bootstrap_servers='kafka:9092')
        self.producer = AIOKafkaProducer(bootstrap_servers='kafka:9092')
        self.redis = redis.Redis(host='redis', port=6379)
        self.buffer = {}

    async def run(self):
        await self.consumer.start()
        await self.producer.start()
        async for msg in self.consumer:
            tick = json.loads(msg.value)
            symbol = tick['symbol']
            price = tick['price']
            key = f"prices:{symbol}"
            data = await self.redis.get(key)
            window = pickle.loads(data) if data else deque(maxlen=200)
            window.append(price)
            await self.redis.set(key, pickle.dumps(window))
            arr = np.array(window, dtype=np.float64)
            if len(arr)>=14:
                features = {
                    'symbol': symbol,
                    'rsi': talib.RSI(arr, timeperiod=14)[-1],
                    'sma': talib.SMA(arr, timeperiod=20)[-1],
                    'norm_price': (price - np.mean(arr))/(np.std(arr)+1e-7),
                }
                await self.producer.send('features.norm', json.dumps(features).encode())

if __name__ == '__main__':
    asyncio.run(FeatureEngine().run())
