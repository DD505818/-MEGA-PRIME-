import threading
from fastapi import FastAPI
from .config import KAFKA_BOOTSTRAP, CONSUME_TOPIC, PRODUCE_TOPIC, SERVICE_NAME
from .kafka_client import build_consumer, build_producer, publish_heartbeat
from . import agent

app = FastAPI(title=SERVICE_NAME)

@app.get("/health")
def health():
    return {"service": SERVICE_NAME, "status": "ok"}

@app.get("/forecast")
def forecast():
    return agent.get_forecast()

def worker():
    producer = build_producer(KAFKA_BOOTSTRAP)
    consumer = build_consumer(KAFKA_BOOTSTRAP, CONSUME_TOPIC, SERVICE_NAME)
    publish_heartbeat(producer, PRODUCE_TOPIC, SERVICE_NAME)
    for msg in consumer:
        signal = agent.generate_signal(msg.value)
        if signal:
            producer.send(PRODUCE_TOPIC, signal)

@app.on_event("startup")
def start_worker():
    threading.Thread(target=worker, daemon=True).start()
