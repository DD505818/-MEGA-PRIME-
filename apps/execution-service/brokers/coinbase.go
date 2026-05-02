// Coinbase Advanced Trade API v3 adapter.
// HMAC-SHA256 signed with api_key + api_secret (JWT variant for v3).
// Sandbox: https://api-public.sandbox.exchange.coinbase.com
// Production: https://api.coinbase.com/api/v3/brokerage
package brokers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	coinbaseProdURL    = "https://api.coinbase.com/api/v3/brokerage"
	coinbaseSandboxURL = "https://api-public.sandbox.exchange.coinbase.com"
)

type CoinbaseAdapter struct {
	apiKey     string
	apiSecret  string
	baseURL    string
	client     *http.Client
}

func NewCoinbaseAdapter(apiKey, apiSecret string, sandbox bool) *CoinbaseAdapter {
	base := coinbaseProdURL
	if sandbox {
		base = coinbaseSandboxURL
	}
	return &CoinbaseAdapter{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   base,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *CoinbaseAdapter) Name() string { return "Coinbase" }

func (c *CoinbaseAdapter) PlaceLimitOrder(ctx context.Context, symbol, side string, qty, price float64) (*OrderResult, error) {
	clientOrderID := fmt.Sprintf("omega-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"client_order_id": clientOrderID,
		"product_id":      symbol,
		"side":            strings.ToUpper(side),
		"order_configuration": map[string]interface{}{
			"limit_limit_gtc": map[string]string{
				"base_size":   strconv.FormatFloat(qty, 'f', 8, 64),
				"limit_price": strconv.FormatFloat(price, 'f', 2, 64),
			},
		},
	}
	raw, err := c.doRequest(ctx, "POST", "/orders", body)
	if err != nil {
		return nil, err
	}
	return c.parseOrderResponse(raw, symbol, side), nil
}

func (c *CoinbaseAdapter) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*OrderResult, error) {
	clientOrderID := fmt.Sprintf("omega-%d", time.Now().UnixNano())
	body := map[string]interface{}{
		"client_order_id": clientOrderID,
		"product_id":      symbol,
		"side":            strings.ToUpper(side),
		"order_configuration": map[string]interface{}{
			"market_market_ioc": map[string]string{
				"base_size": strconv.FormatFloat(qty, 'f', 8, 64),
			},
		},
	}
	raw, err := c.doRequest(ctx, "POST", "/orders", body)
	if err != nil {
		return nil, err
	}
	return c.parseOrderResponse(raw, symbol, side), nil
}

func (c *CoinbaseAdapter) CancelOrder(ctx context.Context, venueOrderID string) error {
	body := map[string]interface{}{
		"order_ids": []string{venueOrderID},
	}
	_, err := c.doRequest(ctx, "POST", "/orders/batch_cancel", body)
	return err
}

func (c *CoinbaseAdapter) CancelAllOrders(ctx context.Context) error {
	// List open orders then batch cancel
	raw, err := c.doRequest(ctx, "GET", "/orders/historical/batch?order_status=OPEN&limit=100", nil)
	if err != nil {
		return err
	}
	var resp struct {
		Orders []struct {
			OrderID string `json:"order_id"`
		} `json:"orders"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return err
	}
	if len(resp.Orders) == 0 {
		return nil
	}
	ids := make([]string, len(resp.Orders))
	for i, o := range resp.Orders {
		ids[i] = o.OrderID
	}
	_, err = c.doRequest(ctx, "POST", "/orders/batch_cancel", map[string]interface{}{"order_ids": ids})
	return err
}

func (c *CoinbaseAdapter) GetOrderStatus(ctx context.Context, venueOrderID string) (*OrderResult, error) {
	raw, err := c.doRequest(ctx, "GET", "/orders/historical/"+venueOrderID, nil)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Order struct {
			OrderID      string `json:"order_id"`
			Status       string `json:"status"`
			FilledSize   string `json:"filled_size"`
			AveragePrice string `json:"average_filled_price"`
			ProductID    string `json:"product_id"`
			Side         string `json:"side"`
		} `json:"order"`
	}
	json.Unmarshal(raw, &wrapper)
	o := wrapper.Order
	filledQty, _ := strconv.ParseFloat(o.FilledSize, 64)
	avgPrice, _ := strconv.ParseFloat(o.AveragePrice, 64)
	return &OrderResult{
		VenueOrderID: o.OrderID,
		Symbol:       o.ProductID,
		Side:         o.Side,
		FilledQty:    filledQty,
		AvgPrice:     avgPrice,
		Status:       o.Status,
		RawResponse:  raw,
	}, nil
}

func (c *CoinbaseAdapter) HealthCheck(ctx context.Context) error {
	_, err := c.doRequest(ctx, "GET", "/products/BTC-USD", nil)
	return err
}

func (c *CoinbaseAdapter) parseOrderResponse(raw []byte, symbol, side string) *OrderResult {
	var resp struct {
		OrderID      string `json:"order_id"`
		Success      bool   `json:"success"`
		FilledSize   string `json:"filled_size"`
		AveragePrice string `json:"average_filled_price"`
	}
	json.Unmarshal(raw, &resp)
	filledQty, _ := strconv.ParseFloat(resp.FilledSize, 64)
	avgPrice, _ := strconv.ParseFloat(resp.AveragePrice, 64)
	status := "PENDING"
	if resp.Success {
		status = "SUBMITTED"
	}
	return &OrderResult{
		VenueOrderID: resp.OrderID,
		Symbol:       symbol,
		Side:         side,
		FilledQty:    filledQty,
		AvgPrice:     avgPrice,
		Status:       status,
		RawResponse:  raw,
	}
}

func (c *CoinbaseAdapter) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var bodyBytes []byte
	var err error
	if body != nil {
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := c.sign(timestamp, method, path, string(bodyBytes))

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("CB-ACCESS-KEY", c.apiKey)
	req.Header.Set("CB-ACCESS-SIGN", signature)
	req.Header.Set("CB-ACCESS-TIMESTAMP", timestamp)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("coinbase %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("coinbase %d: %s", resp.StatusCode, raw)
	}
	return raw, nil
}

func (c *CoinbaseAdapter) sign(timestamp, method, path, body string) string {
	message := timestamp + method + path + body
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}
