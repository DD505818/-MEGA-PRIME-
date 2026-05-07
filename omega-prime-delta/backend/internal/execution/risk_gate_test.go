package execution

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omega-prime/omega-prime-delta/backend/internal/models"
)

func TestRiskGateValidateWrapsOrderPayload(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		var payload struct {
			Order models.Order `json:"order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Order.Symbol != "BTC/USD" {
			t.Fatalf("expected symbol BTC/USD, got %s", payload.Order.Symbol)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	gate := NewRiskGate(ts.URL)
	err := gate.Validate(models.Order{ID: "order-1", Symbol: "BTC/USD", Side: "BUY", Qty: 1})
	if err != nil {
		t.Fatalf("validate returned unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected request to risk service")
	}
}

func TestRiskGateValidateRejectsNon200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "risk breach", http.StatusBadRequest)
	}))
	defer ts.Close()

	gate := NewRiskGate(ts.URL)
	err := gate.Validate(models.Order{ID: "order-2", Symbol: "ETH/USD", Side: "SELL", Qty: 2})
	if err == nil {
		t.Fatal("expected rejection error")
	}
	if err != ErrRiskRejected {
		t.Fatalf("expected ErrRiskRejected, got %v", err)
	}
}
