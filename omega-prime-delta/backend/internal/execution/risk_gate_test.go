package execution

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omega-prime/omega-prime-delta/backend/internal/models"
)

func TestRiskGateValidateSendsOrderEnvelope(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	gate := NewRiskGate(server.URL)
	err := gate.Validate(models.Order{OrderID: "ord-1", Symbol: "AAPL", Qty: 5})
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}

	order, ok := got["order"].(map[string]any)
	if !ok {
		t.Fatalf("expected top-level order object in payload, got %#v", got)
	}
	if order["orderId"] != "ord-1" {
		t.Fatalf("expected order.orderId to be ord-1, got %#v", order["orderId"])
	}
}

func TestRiskGateValidateMapsNon200ToRiskRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	gate := NewRiskGate(server.URL)
	err := gate.Validate(models.Order{OrderID: "ord-2"})
	if !errors.Is(err, ErrRiskRejected) {
		t.Fatalf("expected ErrRiskRejected, got %v", err)
	}
}
