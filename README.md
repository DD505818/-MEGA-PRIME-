# ΩMEGA PRIME Δ PRO

ΩMEGA PRIME Δ is the institutional-grade autonomous trading platform for mobile. 
Monitor your portfolio in real time, deploy AI-generated strategies from natural 
language, and track self-optimizing RL agents—all with bank-grade security.

Key Features:
• Real-time portfolio PnL and allocation
• Live market intelligence across crypto, forex, and indices
• AI Strategy Studio: Create trading bots with plain English
• RL Optimizer: Watch your strategies evolve autonomously
• Emergency Kill Switch with slide-to-confirm
• Biometric authentication (Face ID / Touch ID)
• Push notifications for critical alerts

Built for professional traders, quant funds, and serious investors.Institutional-grade distributed trading platform scaffold with microservices, Kafka event streaming, RL agents, research lab, and Kubernetes deployment.

## Architecture
- **Microservices:** market-data, feature-engine, strategy-engine, rl-agent-cluster, execution-router, risk-engine, portfolio-service, backtesting-cluster.
- **Event backbone:** Kafka topics `market.ticks`, `market.orderbooks`, `features.volatility`, `features.orderflow`, `signals.strategy`, `orders.execution`, `portfolio.decisions`, `risk.alerts`.
- **Execution venues:** Binance, Kraken, Coinbase, Bybit, Alpaca (smart routing by spread/liquidity/latency).
- **Research:** GPU Monte Carlo (50k+ paths), PPO and DQN agent templates, regime detection models.
- **US30 strategies:** liquidity sweep reversal, VWAP continuation, ORB, orderflow imbalance breakout, volatility expansion breakout, liquidity-zone mean reversion.
- **Observability:** Prometheus, Grafana, Alertmanager dashboards and alerts.
- **Ops UI:** modern dark-mode dashboard (`http://localhost:3000`) and control panel (`http://localhost:3001`) with live-ready API endpoints at `/api/widgets` and `/api/actions`.

## Repository layout

This repository keeps experiment output folders (`backtests/`, `cache/`, `data/`, `logs/`, and `models/`) in place, while ignoring generated artifacts inside them.

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
