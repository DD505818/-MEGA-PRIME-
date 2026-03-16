import json
import time
from kafka import KafkaConsumer, KafkaProducer

def build_producer(bootstrap: str):
    return KafkaProducer(
        bootstrap_servers=bootstrap,
        value_serializer=lambda v: json.dumps(v).encode("utf-8"),
    )

def build_consumer(bootstrap: str, topic: str, group_id: str):
    return KafkaConsumer(
        topic,
        bootstrap_servers=bootstrap,
        auto_offset_reset="latest",
        enable_auto_commit=True,
        value_deserializer=lambda m: json.loads(m.decode("utf-8")),
        group_id=group_id,
    )

def publish_heartbeat(producer, topic: str, service: str):
    producer.send(topic, {"event": "heartbeat", "service": service, "ts": time.time()})
