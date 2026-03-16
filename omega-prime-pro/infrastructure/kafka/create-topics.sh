#!/usr/bin/env bash
set -e
for topic in market.ticks market.orderbooks features.volatility features.orderflow signals.strategy orders.execution portfolio.decisions risk.alerts; do
  echo "creating $topic"
done
