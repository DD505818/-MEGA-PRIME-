import threading
from fastapi import FastAPI
from pydantic import BaseModel
from .config import KAFKA_BOOTSTRAP, CONSUME_TOPIC, PRODUCE_TOPIC, SERVICE_NAME
from .kafka_client import build_consumer, build_producer, publish_heartbeat
from . import agent as ceo

app = FastAPI(title=SERVICE_NAME)

@app.get("/health")
def health():
    return {"service": SERVICE_NAME, "status": "ok"}

@app.get("/status")
def status():
    return ceo.get_status()

class AgentAction(BaseModel):
    agent: str

@app.post("/activate")
def activate(body: AgentAction):
    return ceo.activate(body.agent)

@app.post("/deactivate")
def deactivate(body: AgentAction):
    return ceo.deactivate(body.agent)

def worker():
    producer = build_producer(KAFKA_BOOTSTRAP)
    consumer = build_consumer(KAFKA_BOOTSTRAP, CONSUME_TOPIC, SERVICE_NAME)
    publish_heartbeat(producer, PRODUCE_TOPIC, SERVICE_NAME)
    for msg in consumer:
        result = ceo.transform(msg.value)
        producer.send(PRODUCE_TOPIC, result)

@app.on_event("startup")
def start_worker():
    threading.Thread(target=worker, daemon=True).start()
