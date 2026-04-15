import os
import threading
import time
from fastapi import FastAPI

from .config import KAFKA_BOOTSTRAP, SERVICE_NAME
from .kafka_client import build_consumer, build_producer, publish_heartbeat
from . import logic
from .safety import OPERATOR_APPROVAL_REQUIRED

# Topics are configurable so we can wire approvals safely without changing existing config.py.
CONSUME_TOPIC = os.getenv("EXECUTION_CONSUME_TOPIC", os.getenv("CONSUME_TOPIC", "orders.approved"))
PRODUCE_TOPIC = os.getenv("EXECUTION_PRODUCE_TOPIC", os.getenv("PRODUCE_TOPIC", "orders.fills"))
DLQ_TOPIC = os.getenv("EXECUTION_DLQ_TOPIC", "orders.execution.dlq")
ERROR_TOPIC = os.getenv("EXECUTION_ERROR_TOPIC", "risk.alerts")

app = FastAPI(title=f"{SERVICE_NAME}-hardened")


@app.get("/health")
def health():
    return {
        "service": f"{SERVICE_NAME}-hardened",
        "status": "ok",
        "consume": CONSUME_TOPIC,
        "produce": PRODUCE_TOPIC,
        "operator_approval_required": OPERATOR_APPROVAL_REQUIRED,
    }


def worker():
    producer = build_producer(KAFKA_BOOTSTRAP)
    consumer = build_consumer(KAFKA_BOOTSTRAP, CONSUME_TOPIC, f"{SERVICE_NAME}-hardened")

    # Heartbeat stays on the same path as before.
    publish_heartbeat(producer, PRODUCE_TOPIC, f"{SERVICE_NAME}-hardened")

    for message in consumer:
        payload = message.value
        try:
            # ExecutionManager is injected by logic module if present; otherwise fall back to transform.
            if hasattr(logic, "execution_manager"):
                out = logic.execution_manager(payload, producer=producer, dlq_topic=DLQ_TOPIC)
            else:
                out = getattr(logic, "transform", lambda x: x)(payload)

            if out is not None:
                producer.send(PRODUCE_TOPIC, out)
        except Exception as e:
            producer.send(ERROR_TOPIC, {"event": "execution_error", "error": str(e), "payload": payload, "ts": time.time()})
            producer.send(DLQ_TOPIC, {"event": "dlq", "error": str(e), "payload": payload, "ts": time.time()})


@app.on_event("startup")
def start_worker():
    t = threading.Thread(target=worker, daemon=True)
    t.start()
