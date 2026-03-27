import asyncio
import json
from kafka import KafkaConsumer, KafkaProducer
from agents import Agent001, Agent002, Agent003, Agent004, Agent005, Agent006, Agent007, Agent008, Agent009, Agent010, Agent011, Agent012, Agent013, Agent014, Agent015

consumer = KafkaConsumer('market.ticks', bootstrap_servers='kafka:9092')
producer = KafkaProducer(bootstrap_servers='kafka:9092', value_serializer=lambda v: json.dumps(v).encode())
agents = [Agent001(), Agent002(), Agent003(), Agent004(), Agent005(), Agent006(), Agent007(), Agent008(), Agent009(), Agent010(), Agent011(), Agent012(), Agent013(), Agent014(), Agent015()]

async def process():
    for msg in consumer:
        tick = json.loads(msg.value)
        context = {'bars': [], 'equity': 10000}
        for agent in agents:
            signal = agent.on_tick(tick, context)
            if signal:
                producer.send('signals.generated', value=signal)

if __name__ == '__main__':
    asyncio.run(process())
