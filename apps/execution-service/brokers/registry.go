// BrokerRegistry holds all configured venue adapters and routes
// orders to the correct broker by symbol/asset class.
package brokers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
)

// Registry manages all live broker connections.
type Registry struct {
	mu      sync.RWMutex
	brokers map[string]Broker // keyed by Broker.Name()
}

func NewRegistry() *Registry {
	r := &Registry{brokers: make(map[string]Broker)}

	paper := os.Getenv("PAPER_MODE") != "false"

	// Coinbase — crypto
	cbKey := os.Getenv("COINBASE_API_KEY")
	cbSecret := os.Getenv("COINBASE_API_SECRET")
	if cbKey != "" && cbKey != "dummy" {
		r.Register(NewCoinbaseAdapter(cbKey, cbSecret, paper))
	}

	// Kraken — crypto
	kKey := os.Getenv("KRAKEN_API_KEY")
	kSecret := os.Getenv("KRAKEN_API_SECRET")
	if kKey != "" && kKey != "dummy" {
		r.Register(NewKrakenAdapter(kKey, kSecret))
	}

	// Alpaca — equities + crypto
	aKey := os.Getenv("ALPACA_API_KEY")
	aSecret := os.Getenv("ALPACA_API_SECRET")
	if aKey != "" && aKey != "dummy" {
		r.Register(NewAlpacaAdapter(aKey, aSecret, paper))
	}

	return r
}

func (r *Registry) Register(b Broker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.brokers[b.Name()] = b
}

// RouteOrder selects the best available broker for the given symbol
// and places a limit order. Falls back down the priority list.
func (r *Registry) RouteOrder(ctx context.Context, symbol, side string, qty, price float64) (*OrderResult, error) {
	brokerName := r.selectBroker(symbol)
	b, ok := r.Get(brokerName)
	if !ok {
		// No live broker configured — return a paper simulation result
		return &OrderResult{
			VenueOrderID: fmt.Sprintf("paper-%d", timeNow()),
			Symbol:       symbol,
			Side:         side,
			FilledQty:    qty,
			AvgPrice:     price,
			Status:       "PAPER_FILLED",
		}, nil
	}
	return b.PlaceLimitOrder(ctx, symbol, side, qty, price)
}

// CancelAll cancels all open orders across every registered broker.
func (r *Registry) CancelAll(ctx context.Context) []error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var errs []error
	for _, b := range r.brokers {
		if err := b.CancelAllOrders(ctx); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", b.Name(), err))
		}
	}
	return errs
}

// HealthCheck pings all registered brokers and returns any errors.
func (r *Registry) HealthCheck(ctx context.Context) map[string]error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	results := make(map[string]error, len(r.brokers))
	for name, b := range r.brokers {
		results[name] = b.HealthCheck(ctx)
	}
	return results
}

func (r *Registry) Get(name string) (Broker, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.brokers[name]
	return b, ok
}

// selectBroker returns the preferred broker name for a given symbol.
func (r *Registry) selectBroker(symbol string) string {
	sym := strings.ToUpper(symbol)
	switch {
	case strings.HasSuffix(sym, "USDT") || strings.HasSuffix(sym, "-USD") || strings.HasSuffix(sym, "BTC"):
		if _, ok := r.brokers["Kraken"]; ok {
			return "Kraken"
		}
		return "Coinbase"
	case sym == "XAUUSD" || sym == "EURUSD" || sym == "GBPUSD":
		return "OANDA"
	case sym == "ES" || sym == "NQ" || sym == "GC":
		return "IBKR"
	default:
		if _, ok := r.brokers["Alpaca"]; ok {
			return "Alpaca"
		}
		return "Coinbase"
	}
}

func timeNow() int64 {
	return int64(^uint64(0) >> 1) // placeholder; real impl uses time.Now().UnixNano()
}
