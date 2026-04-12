package brokers

import "log/slog"

func NewKraken(config Config, logger *slog.Logger) Adapter {
	return newSimulatedAdapter("kraken", config, logger)
}
