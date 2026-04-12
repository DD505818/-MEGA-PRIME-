import os
KAFKA_BOOTSTRAP = os.getenv("KAFKA_BOOTSTRAP", "kafka:9092")
CONSUME_TOPIC   = os.getenv("CONSUME_TOPIC",   "features.volatility")
PRODUCE_TOPIC   = os.getenv("PRODUCE_TOPIC",   "agents.surge.signals")
SERVICE_NAME    = "agent-surge"
