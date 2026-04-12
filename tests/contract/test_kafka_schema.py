import json

import pytest

try:
    from confluent_kafka.schema_registry import SchemaRegistryClient
except ImportError:  # pragma: no cover - optional dependency
    SchemaRegistryClient = None


SCHEMA_REGISTRY_URL = "http://localhost:8081/apis/registry/v2"


@pytest.mark.contract
def test_order_event_schema_is_backwards_compatible():
    if SchemaRegistryClient is None:
        pytest.skip("confluent-kafka is not installed")

    client = SchemaRegistryClient({"url": SCHEMA_REGISTRY_URL})
    subject = "omega.trades-value"

    evolved_schema = json.dumps(
        {
            "type": "record",
            "name": "OrderEvent",
            "namespace": "omega.trades",
            "fields": [
                {"name": "event_id", "type": "string"},
                {
                    "name": "event_type",
                    "type": {
                        "type": "enum",
                        "name": "EventType",
                        "symbols": [
                            "ORDER_PLACED",
                            "ORDER_FILLED",
                            "ORDER_CANCELLED",
                        ],
                    },
                },
                {"name": "aggregate_id", "type": "string"},
                {"name": "timestamp", "type": "string"},
                {"name": "version", "type": "int", "default": 1},
                {"name": "data", "type": {"type": "map", "values": "string"}},
                {
                    "name": "metadata",
                    "type": ["null", {"type": "map", "values": "string"}],
                    "default": None,
                },
            ],
        }
    )

    is_compatible = client.test_compatibility(subject, evolved_schema)
    assert is_compatible
