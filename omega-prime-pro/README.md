# ΩMEGA PRIME Δ PRO

Institutional-grade distributed trading platform with Kafka event backbone, multi-exchange execution routing, AI strategy agents, and full risk controls.

## Core Capabilities
- Distributed microservices with async event-driven workflows.
- Kafka topics: `market.ticks`, `market.orderbooks`, `features.volatility`, `features.orderflow`, `signals.strategy`, `orders.execution`, `portfolio.decisions`, `risk.alerts`.
- RL agents (PPO, DQN), Monte Carlo GPU research cluster (50k–100k paths), and 50+ strategy research modules.
- Multi-exchange execution via CCXT (Binance, Kraken, Coinbase, Bybit, Alpaca).
- Risk constraints: 0.5% risk per trade, 2% daily loss, 10% max drawdown, 25% max exposure with trading halt protection.
- Local development via docker compose and production deployment via Kubernetes.
