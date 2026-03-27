package brokers

import "log/slog"

func NewIBKR(config Config, logger *slog.Logger) Adapter {
	return newSimulatedAdapter("ibkr", config, logger)
}
