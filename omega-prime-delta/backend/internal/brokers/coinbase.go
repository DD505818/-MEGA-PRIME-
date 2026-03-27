package brokers

import "log/slog"

func NewCoinbase(config Config, logger *slog.Logger) Adapter {
	return newSimulatedAdapter("coinbase", config, logger)
}
