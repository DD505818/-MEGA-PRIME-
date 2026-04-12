import os
KAFKA_BOOTSTRAP = os.getenv("KAFKA_BOOTSTRAP", "kafka:9092")
CONSUME_TOPIC   = os.getenv("CONSUME_TOPIC",   "risk.alerts")
PRODUCE_TOPIC   = os.getenv("PRODUCE_TOPIC",   "agents.guard.signals")
SERVICE_NAME    = "agent-guard"
