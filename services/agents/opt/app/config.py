import os
KAFKA_BOOTSTRAP = os.getenv("KAFKA_BOOTSTRAP", "kafka:9092")
CONSUME_TOPIC   = os.getenv("CONSUME_TOPIC",   "features.orderflow")
PRODUCE_TOPIC   = os.getenv("PRODUCE_TOPIC",   "agents.opt.signals")
SERVICE_NAME    = "agent-opt"
