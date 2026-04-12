import os
KAFKA_BOOTSTRAP = os.getenv("KAFKA_BOOTSTRAP", "kafka:9092")
CONSUME_TOPIC   = os.getenv("CONSUME_TOPIC",   "agents.cfo.budgets")
PRODUCE_TOPIC   = os.getenv("PRODUCE_TOPIC",   "agents.ceo.commands")
SERVICE_NAME    = "agent-ceo"
