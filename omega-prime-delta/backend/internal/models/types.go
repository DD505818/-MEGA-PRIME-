package models

import "time"

type Order struct {
    OrderID    string    `json:"orderId"`
    Symbol     string    `json:"symbol"`
    Side       string    `json:"side"`
    Qty        float64   `json:"qty"`
    Type       string    `json:"type"`
    Price      float64   `json:"price,omitempty"`
    Agent      string    `json:"agent"`
    Strategy   string    `json:"strategy"`
    Confidence float64   `json:"confidence"`
    Timestamp  time.Time `json:"timestamp"`
}

type Fill struct {
    OrderID    string    `json:"orderId"`
    Symbol     string    `json:"symbol"`
    Side       string    `json:"side"`
    Qty        float64   `json:"qty"`
    Price      float64   `json:"price"`
    Commission float64   `json:"commission"`
    Agent      string    `json:"agent"`
    Strategy   string    `json:"strategy"`
    Timestamp  time.Time `json:"timestamp"`
}

type PortfolioState struct {
    Equity     float64   `json:"equity"`
    DailyLoss  float64   `json:"dailyLoss"`
    PeakEquity float64   `json:"peakEquity"`
    Drawdown   float64   `json:"drawdown"`
    UpdatedAt  time.Time `json:"updatedAt"`
}
