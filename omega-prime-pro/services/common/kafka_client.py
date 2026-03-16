from dataclasses import dataclass

@dataclass
class KafkaConfig:
    broker: str = "kafka:9092"

class EventBus:
    def __init__(self, cfg: KafkaConfig):
        self.cfg = cfg

    def publish(self, topic: str, payload: dict) -> None:
        print(f"publish::{topic}::{payload}")

    def consume(self, topic: str) -> None:
        print(f"consume::{topic}")
