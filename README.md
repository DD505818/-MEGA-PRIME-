# ΩMEGA PRIME Δ

**Risk-first, multi-agent, multi-asset autonomous trading platform.**

## Quickstart

```bash
cp .env.example .env
make up
```

Services

· market-data-service
· feature-engine
· agent-service
· risk-service
· execution-service
· capital-allocator
· portfolio-service
· truth-core
· websocket-gateway
· web-ui
· llm-service

Architecture

```
market-data (Rust) -> feature-engine (Python) -> agent-service (Python)
                                              ↓
                                      signals.raw (Kafka)
                                              ↓
                                      risk-service (Go)
                                              ↓
                                      orders.cmd (Kafka)
                                              ↓
                                      execution-service (Go)
                                              ↓
                                      portfolio-service (Go) + truth-core (Go)
                                              ↓
                                      websocket-gateway -> web-ui
```

Testing

```bash
make test
```

Safety Warning

Paper mode is enabled by default. No live orders will be placed without explicit configuration and broker adapter setup. This platform requires independent validation before managing real capital.
