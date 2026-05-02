// Broker interface — all venue adapters implement this contract.
package brokers

import "context"

// OrderResult is returned after a successful placement.
type OrderResult struct {
	VenueOrderID string
	Symbol       string
	Side         string
	FilledQty    float64
	AvgPrice     float64
	Status       string
	RawResponse  []byte
}

// Broker is the uniform interface every venue adapter must satisfy.
type Broker interface {
	// Name returns the canonical venue identifier.
	Name() string

	// PlaceLimitOrder submits a limit order and returns the result.
	PlaceLimitOrder(ctx context.Context, symbol, side string, qty, price float64) (*OrderResult, error)

	// PlaceMarketOrder submits a market order.
	PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*OrderResult, error)

	// CancelOrder cancels an open order by venue order ID.
	CancelOrder(ctx context.Context, venueOrderID string) error

	// CancelAllOrders cancels every open order (used by kill cascade).
	CancelAllOrders(ctx context.Context) error

	// GetOrderStatus returns the current state of an order.
	GetOrderStatus(ctx context.Context, venueOrderID string) (*OrderResult, error)

	// HealthCheck returns nil if the venue is reachable.
	HealthCheck(ctx context.Context) error
}
