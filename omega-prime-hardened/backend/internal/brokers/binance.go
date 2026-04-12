package brokers

import "log/slog"

func NewBinance(config Config, logger *slog.Logger) Adapter {
	return newSimulatedAdapter("binance", config, logger)
}
