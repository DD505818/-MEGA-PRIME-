import json
import os
import threading
import time
import uuid
from datetime import datetime

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

from kafka import KafkaConsumer, KafkaProducer


SERVICE_NAME = "execution-control-service"
KAFKA_BOOTSTRAP = os.getenv("KAFKA_BOOTSTRAP", "kafka:9092")
PROPOSED_TOPIC = os.getenv("PROPOSED_TOPIC", "orders.proposed")
PENDING_TOPIC = os.getenv("PENDING_TOPIC", "orders.pending_approval")
APPROVED_TOPIC = os.getenv("APPROVED_TOPIC", "orders.approved")
REJECTED_TOPIC = os.getenv("REJECTED_TOPIC", "orders.rejected")

LIVE_EXECUTION_ENABLED = os.getenv("LIVE_EXECUTION_ENABLED", "false").lower() == "true"
OPERATOR_APPROVAL_REQUIRED = os.getenv("OPERATOR_APPROVAL_REQUIRED", "true").lower() == "true"

APPROVAL_JOURNAL_PATH = os.getenv("APPROVAL_JOURNAL_PATH", "/tmp/omega/approvals.journal")

app = FastAPI(title=SERVICE_NAME)


class OrderProposal(BaseModel):
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    order: dict
    requested_by: str | None = None
    requested_at: str = Field(default_factory=lambda: datetime.utcnow().isoformat() + "Z")


class Decision(BaseModel):
    id: str
    approved_by: str
    reason: str | None = None


_pending: dict[str, OrderProposal] = {}
_lock = threading.Lock()


def _append_journal(event: dict) -> None:
    os.makedirs(os.path.dirname(APPROVAL_JOURNAL_PATH), exist_ok=True)
    line = json.dumps(event, separators=(",", ":"), sort_keys=True)
    with open(APPROVAL_JOURNAL_PATH, "a", encoding="utf-8") as f:
        f.write(line + "\n")
        f.flush()
        os.fsync(f.fileno())


def _build_producer() -> KafkaProducer:
    return KafkaProducer(
        bootstrap_servers=KAFKA_BOOTSTRAP,
        value_serializer=lambda v: json.dumps(v).encode("utf-8"),
    )


def _build_consumer() -> KafkaConsumer:
    return KafkaConsumer(
        PROPOSED_TOPIC,
        bootstrap_servers=KAFKA_BOOTSTRAP,
        auto_offset_reset="latest",
        enable_auto_commit=True,
        value_deserializer=lambda m: json.loads(m.decode("utf-8")),
        group_id=SERVICE_NAME,
    )


def _gate_worker() -> None:
    producer = _build_producer()
    consumer = _build_consumer()

    # Fail-closed: if operator approval is required, never forward to APPROVED_TOPIC
    # without explicit operator API action.
    for msg in consumer:
        payload = msg.value

        if not OPERATOR_APPROVAL_REQUIRED:
            # Still fail-closed when live execution is disabled.
            if not LIVE_EXECUTION_ENABLED:
                producer.send(REJECTED_TOPIC, {
                    "event": "rejected",
                    "reason": "live_execution_disabled",
                    "payload": payload,
                    "ts": time.time(),
                })
                continue

            producer.send(APPROVED_TOPIC, {
                "event": "auto_approved",
                "payload": payload,
                "ts": time.time(),
            })
            continue

        proposal = OrderProposal(order=payload.get("order", payload), requested_by=payload.get("requested_by"))
        with _lock:
            _pending[proposal.id] = proposal

        evt = {
            "event": "pending",
            "id": proposal.id,
            "order": proposal.order,
            "requested_by": proposal.requested_by,
            "requested_at": proposal.requested_at,
            "ts": time.time(),
        }
        _append_journal(evt)
        producer.send(PENDING_TOPIC, evt)


@app.get("/health")
def health():
    return {
        "service": SERVICE_NAME,
        "status": "ok",
        "live_execution_enabled": LIVE_EXECUTION_ENABLED,
        "operator_approval_required": OPERATOR_APPROVAL_REQUIRED,
        "pending_count": len(_pending),
    }


@app.get("/approvals")
def list_pending():
    with _lock:
        return [p.model_dump() for p in _pending.values()]


@app.post("/approvals/{approval_id}/approve")
def approve(approval_id: str, decision: Decision):
    producer = _build_producer()

    with _lock:
        proposal = _pending.pop(approval_id, None)

    if proposal is None:
        raise HTTPException(status_code=404, detail="approval_not_found")

    if not LIVE_EXECUTION_ENABLED:
        # Fail-closed for safety: do not forward to execution topics in disabled mode.
        evt = {
            "event": "rejected",
            "id": approval_id,
            "reason": "live_execution_disabled",
            "approved_by": decision.approved_by,
            "ts": time.time(),
        }
        _append_journal(evt)
        producer.send(REJECTED_TOPIC, {"event": "rejected", "id": approval_id, "payload": proposal.order, **evt})
        raise HTTPException(status_code=409, detail="live_execution_disabled")

    evt = {
        "event": "approved",
        "id": approval_id,
        "approved_by": decision.approved_by,
        "reason": decision.reason,
        "ts": time.time(),
    }
    _append_journal(evt)
    producer.send(APPROVED_TOPIC, {"event": "approved", "id": approval_id, "order": proposal.order, **evt})
    return {"ok": True}


@app.post("/approvals/{approval_id}/reject")
def reject(approval_id: str, decision: Decision):
    producer = _build_producer()

    with _lock:
        proposal = _pending.pop(approval_id, None)

    if proposal is None:
        raise HTTPException(status_code=404, detail="approval_not_found")

    evt = {
        "event": "rejected",
        "id": approval_id,
        "approved_by": decision.approved_by,
        "reason": decision.reason,
        "ts": time.time(),
    }
    _append_journal(evt)
    producer.send(REJECTED_TOPIC, {"event": "rejected", "id": approval_id, "order": proposal.order, **evt})
    return {"ok": True}


@app.on_event("startup")
def start_worker():
    t = threading.Thread(target=_gate_worker, daemon=True)
    t.start()
