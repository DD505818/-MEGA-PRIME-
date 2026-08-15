import os
import ssl


def kafka_client_kwargs() -> dict:
    protocol = os.getenv("KAFKA_SECURITY_PROTOCOL", "PLAINTEXT").upper()
    kwargs: dict = {
        "bootstrap_servers": os.getenv("KAFKA_BROKERS", "kafka:9092"),
        "security_protocol": protocol,
    }
    if protocol in {"SSL", "SASL_SSL"}:
        ca_file = os.getenv("KAFKA_SSL_CA_LOCATION")
        kwargs["ssl_context"] = ssl.create_default_context(cafile=ca_file or None)
    if protocol in {"SASL_PLAINTEXT", "SASL_SSL"}:
        kwargs.update(
            sasl_mechanism=os.getenv("KAFKA_SASL_MECHANISM", "PLAIN"),
            sasl_plain_username=os.environ["KAFKA_SASL_USERNAME"],
            sasl_plain_password=os.environ["KAFKA_SASL_PASSWORD"],
        )
    return kwargs
