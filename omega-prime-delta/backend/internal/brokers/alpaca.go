package brokers

import "log/slog"

func NewAlpaca(config Config, logger *slog.Logger) Adapter {
	return newSimulatedAdapter("alpaca", config, logger)
}
