package brokers

import "log/slog"

func NewOanda(config Config, logger *slog.Logger) Adapter {
	return newSimulatedAdapter("oanda", config, logger)
}
