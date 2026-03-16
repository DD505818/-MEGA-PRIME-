import os
from services.common.kafka_client import EventBus, KafkaConfig

SERVICE = "backtesting-cluster"

def main() -> None:
    bus = EventBus(KafkaConfig(os.getenv("KAFKA_BROKER", "kafka:9092")))
    bus.consume("market.ticks")
    bus.publish("signals.strategy", {"service": SERVICE, "status": "ready"})

if __name__ == "__main__":
    main()
