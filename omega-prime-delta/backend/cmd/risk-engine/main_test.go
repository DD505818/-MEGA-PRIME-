package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestValidateHandlerReloadsStateEveryRequest(t *testing.T) {
	t.Cleanup(func() {
		leadershipMu.Lock()
		isLeader = false
		leadershipMu.Unlock()
	})

	leadershipMu.Lock()
	isLeader = true
	leadershipMu.Unlock()

	origLoader := loadRiskState
	defer func() { loadRiskState = origLoader }()

	var calls int32
	loadRiskState = func(context.Context) (riskState, error) {
		if atomic.AddInt32(&calls, 1) == 1 {
			return riskState{equity: 1000, dailyLoss: -10, drawdown: 0.01}, nil
		}
		return riskState{equity: 1000, dailyLoss: -10, drawdown: 0.2}, nil
	}

	body := []byte(`{"order":{"orderId":"1","symbol":"BTCUSD","side":"buy","qty":1,"type":"limit","price":100,"agent":"Agent001"}}`)

	req1 := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
	rr1 := httptest.NewRecorder()
	validateHandler(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first validate status = %d, want %d", rr1.Code, http.StatusOK)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(body))
	rr2 := httptest.NewRecorder()
	validateHandler(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("second validate status = %d, want %d", rr2.Code, http.StatusBadRequest)
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("loadRiskState calls = %d, want 2", got)
	}
}
