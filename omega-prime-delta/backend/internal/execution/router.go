package execution

import "errors"

type VenueQuote struct {
	Venue     string
	SpreadBps float64
	Liquidity float64
	LatencyMS float64
}

func RouteOrder(candidates []VenueQuote) (VenueQuote, error) {
	if len(candidates) == 0 {
		return VenueQuote{}, errors.New("at least one venue quote is required")
	}

	best := candidates[0]
	for _, quote := range candidates[1:] {
		if isBetterQuote(quote, best) {
			best = quote
		}
	}
	return best, nil
}

func isBetterQuote(current, best VenueQuote) bool {
	if current.SpreadBps != best.SpreadBps {
		return current.SpreadBps < best.SpreadBps
	}
	if current.Liquidity != best.Liquidity {
		return current.Liquidity > best.Liquidity
	}
	return current.LatencyMS < best.LatencyMS
}
