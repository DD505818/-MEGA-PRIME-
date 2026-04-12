package execution

import "fmt"

type Route struct {
	Broker string
	Venue  string
}

// SelectRoute chooses a deterministic execution route per symbol family.
func SelectRoute(symbol string) (Route, error) {
	if symbol == "" {
		return Route{}, fmt.Errorf("symbol required")
	}
	switch {
	case len(symbol) >= 4 && symbol[len(symbol)-4:] == "USDT":
		return Route{Broker: "binance", Venue: "crypto"}, nil
	case len(symbol) >= 3 && symbol[len(symbol)-3:] == "USD":
		return Route{Broker: "coinbase", Venue: "crypto"}, nil
	default:
		return Route{Broker: "ibkr", Venue: "equities"}, nil
	}
}
