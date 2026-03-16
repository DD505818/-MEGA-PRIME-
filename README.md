# ΩMEGA PRIME Δ PRO

Institutional-grade distributed trading platform scaffold with microservices, Kafka event streaming, RL agents, research lab, and Kubernetes deployment.

## Architecture
- **Microservices:** market-data, feature-engine, strategy-engine, rl-agent-cluster, execution-router, risk-engine, portfolio-service, backtesting-cluster.
- **Event backbone:** Kafka topics `market.ticks`, `market.orderbooks`, `features.volatility`, `features.orderflow`, `signals.strategy`, `orders.execution`, `portfolio.decisions`, `risk.alerts`.
- **Execution venues:** Binance, Kraken, Coinbase, Bybit, Alpaca (smart routing by spread/liquidity/latency).
- **Research:** GPU Monte Carlo (50k+ paths), PPO and DQN agent templates, regime detection models.
- **US30 strategies:** liquidity sweep reversal, VWAP continuation, ORB, orderflow imbalance breakout, volatility expansion breakout, liquidity-zone mean reversion.
- **Observability:** Prometheus, Grafana, Alertmanager dashboards and alerts.

## Run locally
```bash
docker compose up --build
```

## Kubernetes
```bash
kubectl apply -f infrastructure/kubernetes/base
kubectl apply -f infrastructure/kubernetes/services
kubectl apply -f infrastructure/kubernetes/monitoring
```

## Risk controls
- `risk_per_trade`: 0.5%
- `daily_loss_limit`: 2%
- `max_drawdown`: 10%
- `max_exposure`: 25%

Breaches trigger trading halt in `risk-engine`.
