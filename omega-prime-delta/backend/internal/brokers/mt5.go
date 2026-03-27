package brokers

import "log/slog"

func NewMT5(config Config, logger *slog.Logger) Adapter {
	return newSimulatedAdapter("mt5", config, logger)
}
