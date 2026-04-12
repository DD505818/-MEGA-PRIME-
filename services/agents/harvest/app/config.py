import os
KAFKA_BOOTSTRAP = os.getenv("KAFKA_BOOTSTRAP", "kafka:9092")
CONSUME_TOPIC   = os.getenv("CONSUME_TOPIC",   "market.ticks")
PRODUCE_TOPIC   = os.getenv("PRODUCE_TOPIC",   "agents.harvest.signals")
SERVICE_NAME    = "agent-harvest"
