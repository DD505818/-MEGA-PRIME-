import threading
from fastapi import FastAPI
from .config import KAFKA_BOOTSTRAP, CONSUME_TOPIC, PRODUCE_TOPIC, SERVICE_NAME
from .kafka_client import build_consumer, build_producer, publish_heartbeat
from . import agent as cfo

app = FastAPI(title=SERVICE_NAME)

@app.get("/health")
def health():
    return {"service": SERVICE_NAME, "status": "ok"}

@app.get("/budget")
def budget():
    return cfo.get_budget()

@app.post("/allocate")
def allocate(body: dict):
    return cfo.allocate(body.get("agents", []))

def worker():
    producer = build_producer(KAFKA_BOOTSTRAP)
    consumer = build_consumer(KAFKA_BOOTSTRAP, CONSUME_TOPIC, SERVICE_NAME)
    publish_heartbeat(producer, PRODUCE_TOPIC, SERVICE_NAME)
    for msg in consumer:
        result = cfo.transform(msg.value)
        producer.send(PRODUCE_TOPIC, result)

@app.on_event("startup")
def start_worker():
    threading.Thread(target=worker, daemon=True).start()
