// Kraken REST API v0 adapter.
// Authentication: HMAC-SHA512 over (nonce + postdata) with base64-decoded API secret.
// Sandbox: https://demo-futures.kraken.com  (this uses the prod endpoint with paper creds)
// Production: https://api.kraken.com
package brokers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const krakenBaseURL = "https://api.kraken.com"

type KrakenAdapter struct {
	apiKey    string
	apiSecret string // base64-encoded
	client    *http.Client
}

func NewKrakenAdapter(apiKey, apiSecret string) *KrakenAdapter {
	return &KrakenAdapter{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (k *KrakenAdapter) Name() string { return "Kraken" }

func (k *KrakenAdapter) PlaceLimitOrder(ctx context.Context, symbol, side string, qty, price float64) (*OrderResult, error) {
	params := url.Values{
		"pair":      {symbol},
		"type":      {strings.ToLower(side)},
		"ordertype": {"limit"},
		"price":     {strconv.FormatFloat(price, 'f', 2, 64)},
		"volume":    {strconv.FormatFloat(qty, 'f', 8, 64)},
	}
	raw, err := k.privatePost(ctx, "/0/private/AddOrder", params)
	if err != nil {
		return nil, err
	}
	return k.parseAddOrderResponse(raw, symbol, side), nil
}

func (k *KrakenAdapter) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*OrderResult, error) {
	params := url.Values{
		"pair":      {symbol},
		"type":      {strings.ToLower(side)},
		"ordertype": {"market"},
		"volume":    {strconv.FormatFloat(qty, 'f', 8, 64)},
	}
	raw, err := k.privatePost(ctx, "/0/private/AddOrder", params)
	if err != nil {
		return nil, err
	}
	return k.parseAddOrderResponse(raw, symbol, side), nil
}

func (k *KrakenAdapter) CancelOrder(ctx context.Context, venueOrderID string) error {
	params := url.Values{"txid": {venueOrderID}}
	_, err := k.privatePost(ctx, "/0/private/CancelOrder", params)
	return err
}

func (k *KrakenAdapter) CancelAllOrders(ctx context.Context) error {
	_, err := k.privatePost(ctx, "/0/private/CancelAll", url.Values{})
	return err
}

func (k *KrakenAdapter) GetOrderStatus(ctx context.Context, venueOrderID string) (*OrderResult, error) {
	params := url.Values{"txid": {venueOrderID}}
	raw, err := k.privatePost(ctx, "/0/private/QueryOrders", params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Error  []string                       `json:"error"`
		Result map[string]krakenOrderInfo     `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	if info, ok := resp.Result[venueOrderID]; ok {
		filledQty, _ := strconv.ParseFloat(info.Vol_exec, 64)
		avgPrice, _ := strconv.ParseFloat(info.Price, 64)
		return &OrderResult{
			VenueOrderID: venueOrderID,
			FilledQty:    filledQty,
			AvgPrice:     avgPrice,
			Status:       info.Status,
			RawResponse:  raw,
		}, nil
	}
	return nil, fmt.Errorf("order %s not found", venueOrderID)
}

func (k *KrakenAdapter) HealthCheck(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", krakenBaseURL+"/0/public/SystemStatus", nil)
	resp, err := k.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("kraken health: %d", resp.StatusCode)
	}
	return nil
}

type krakenOrderInfo struct {
	Status   string `json:"status"`
	Vol_exec string `json:"vol_exec"`
	Price    string `json:"price"`
}

func (k *KrakenAdapter) parseAddOrderResponse(raw []byte, symbol, side string) *OrderResult {
	var resp struct {
		Error  []string `json:"error"`
		Result struct {
			TxIDs []string `json:"txid"`
		} `json:"result"`
	}
	json.Unmarshal(raw, &resp)
	id := ""
	if len(resp.Result.TxIDs) > 0 {
		id = resp.Result.TxIDs[0]
	}
	status := "SUBMITTED"
	if len(resp.Error) > 0 {
		status = "ERROR: " + resp.Error[0]
	}
	return &OrderResult{
		VenueOrderID: id,
		Symbol:       symbol,
		Side:         side,
		Status:       status,
		RawResponse:  raw,
	}
}

func (k *KrakenAdapter) privatePost(ctx context.Context, path string, params url.Values) ([]byte, error) {
	nonce := strconv.FormatInt(time.Now().UnixNano(), 10)
	params.Set("nonce", nonce)
	postData := params.Encode()

	sig, err := k.signature(path, nonce, postData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		krakenBaseURL+path,
		strings.NewReader(postData),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("API-Key", k.apiKey)
	req.Header.Set("API-Sign", sig)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kraken %s: %w", path, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("kraken %d: %s", resp.StatusCode, raw)
	}

	// Surface Kraken API-level errors
	var errCheck struct {
		Error []string `json:"error"`
	}
	if json.Unmarshal(raw, &errCheck) == nil && len(errCheck.Error) > 0 {
		return nil, fmt.Errorf("kraken API error: %v", errCheck.Error)
	}
	return raw, nil
}

// signature computes HMAC-SHA512(SHA256(nonce+postdata), base64_decode(secret))
// as required by Kraken's private endpoint authentication spec.
func (k *KrakenAdapter) signature(urlPath, nonce, postData string) (string, error) {
	secretBytes, err := base64.StdEncoding.DecodeString(k.apiSecret)
	if err != nil {
		return "", fmt.Errorf("kraken secret decode: %w", err)
	}

	// SHA-256 of (nonce + postdata)
	h256 := sha256.New()
	h256.Write([]byte(nonce + postData))
	sha := h256.Sum(nil)

	// HMAC-SHA512 of (urlPath + sha256_bytes)
	mac := hmac.New(sha512.New, secretBytes)
	mac.Write([]byte(urlPath))
	mac.Write(sha)

	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}
