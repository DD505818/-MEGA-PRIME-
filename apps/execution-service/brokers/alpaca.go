// Alpaca Markets adapter — equities and crypto via REST v2.
// Paper trading: https://paper-api.alpaca.markets
// Live trading:  https://api.alpaca.markets
// Auth: APCA-API-KEY-ID + APCA-API-SECRET-KEY headers (no HMAC required).
package brokers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	alpacaPaperURL = "https://paper-api.alpaca.markets"
	alpacaLiveURL  = "https://api.alpaca.markets"
)

type AlpacaAdapter struct {
	apiKey    string
	apiSecret string
	baseURL   string
	client    *http.Client
}

func NewAlpacaAdapter(apiKey, apiSecret string, paper bool) *AlpacaAdapter {
	base := alpacaLiveURL
	if paper {
		base = alpacaPaperURL
	}
	return &AlpacaAdapter{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   base,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (a *AlpacaAdapter) Name() string { return "Alpaca" }

func (a *AlpacaAdapter) PlaceLimitOrder(ctx context.Context, symbol, side string, qty, price float64) (*OrderResult, error) {
	body := map[string]interface{}{
		"symbol":        symbol,
		"side":          strings.ToLower(side),
		"type":          "limit",
		"time_in_force": "day",
		"qty":           strconv.FormatFloat(qty, 'f', 8, 64),
		"limit_price":   strconv.FormatFloat(price, 'f', 2, 64),
	}
	raw, err := a.doRequest(ctx, "POST", "/v2/orders", body)
	if err != nil {
		return nil, err
	}
	return a.parseOrderResponse(raw), nil
}

func (a *AlpacaAdapter) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*OrderResult, error) {
	body := map[string]interface{}{
		"symbol":        symbol,
		"side":          strings.ToLower(side),
		"type":          "market",
		"time_in_force": "day",
		"qty":           strconv.FormatFloat(qty, 'f', 8, 64),
	}
	raw, err := a.doRequest(ctx, "POST", "/v2/orders", body)
	if err != nil {
		return nil, err
	}
	return a.parseOrderResponse(raw), nil
}

func (a *AlpacaAdapter) CancelOrder(ctx context.Context, venueOrderID string) error {
	_, err := a.doRequest(ctx, "DELETE", "/v2/orders/"+venueOrderID, nil)
	return err
}

func (a *AlpacaAdapter) CancelAllOrders(ctx context.Context) error {
	_, err := a.doRequest(ctx, "DELETE", "/v2/orders", nil)
	return err
}

func (a *AlpacaAdapter) GetOrderStatus(ctx context.Context, venueOrderID string) (*OrderResult, error) {
	raw, err := a.doRequest(ctx, "GET", "/v2/orders/"+venueOrderID, nil)
	if err != nil {
		return nil, err
	}
	return a.parseOrderResponse(raw), nil
}

func (a *AlpacaAdapter) HealthCheck(ctx context.Context) error {
	_, err := a.doRequest(ctx, "GET", "/v2/clock", nil)
	return err
}

func (a *AlpacaAdapter) parseOrderResponse(raw []byte) *OrderResult {
	var o struct {
		ID            string `json:"id"`
		Symbol        string `json:"symbol"`
		Side          string `json:"side"`
		FilledQty     string `json:"filled_qty"`
		FilledAvgPrice string `json:"filled_avg_price"`
		Status        string `json:"status"`
	}
	json.Unmarshal(raw, &o)
	filledQty, _ := strconv.ParseFloat(o.FilledQty, 64)
	avgPrice, _ := strconv.ParseFloat(o.FilledAvgPrice, 64)
	return &OrderResult{
		VenueOrderID: o.ID,
		Symbol:       o.Symbol,
		Side:         o.Side,
		FilledQty:    filledQty,
		AvgPrice:     avgPrice,
		Status:       strings.ToUpper(o.Status),
		RawResponse:  raw,
	}
}

func (a *AlpacaAdapter) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var bodyReader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, a.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", a.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", a.apiSecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	// 204 No Content on successful DELETE
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("alpaca %d: %s", resp.StatusCode, raw)
	}
	return raw, nil
}
