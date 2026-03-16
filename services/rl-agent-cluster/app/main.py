import threading
from fastapi import FastAPI
from .config import KAFKA_BOOTSTRAP, CONSUME_TOPIC, PRODUCE_TOPIC, SERVICE_NAME
from .kafka_client import build_consumer, build_producer, publish_heartbeat
from . import logic

app = FastAPI(title=SERVICE_NAME)

@app.get("/health")
def health():
    return {"service": SERVICE_NAME, "status": "ok", "consume": CONSUME_TOPIC, "produce": PRODUCE_TOPIC}

def worker():
    producer = build_producer(KAFKA_BOOTSTRAP)
    consumer = build_consumer(KAFKA_BOOTSTRAP, CONSUME_TOPIC, SERVICE_NAME)
    publish_heartbeat(producer, PRODUCE_TOPIC, SERVICE_NAME)
    for message in consumer:
        payload = message.value
        transformed = getattr(logic, "transform", lambda x: x)(payload)
        producer.send(PRODUCE_TOPIC, transformed)

@app.on_event("startup")
def start_worker():
    t = threading.Thread(target=worker, daemon=True)
    t.start()
