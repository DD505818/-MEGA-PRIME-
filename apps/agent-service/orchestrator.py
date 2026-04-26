import asyncio, json, os
from aiokafka import AIOKafkaConsumer, AIOKafkaProducer
import pandas as pd
from normalizer import normalize_signal

class Orchestrator:
    def __init__(self, strategies):
        self.strategies = {s.name: s for s in strategies}

    async def run(self):
        producer = AIOKafkaProducer(bootstrap_servers='kafka:9092')
        await producer.start()
        consumer = AIOKafkaConsumer('features.norm', bootstrap_servers='kafka:9092', group_id='agent-service')
        await consumer.start()
        daily = pd.DataFrame({'high': [63500,63550], 'low':[63300,63350], 'close':[63400,63450]})
        intraday = pd.DataFrame({'high': [63500,63510], 'low':[63480,63490], 'close':[63490,63495], 'open':[63485,63492]})
        async for msg in consumer:
            for strat in self.strategies.values():
                raw = strat.generate_signal(daily, intraday)
                signal = normalize_signal(raw, strat.name)
                if signal:
                    await producer.send('signals.raw', json.dumps(signal).encode())
