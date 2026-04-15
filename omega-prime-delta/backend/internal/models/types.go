package models

import "time"

type Order struct {
	ID         string    `json:"id,omitempty"`
	OrderID    string    `json:"orderId,omitempty"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	Qty        float64   `json:"qty"`
	Type       string    `json:"type,omitempty"`
	Price      float64   `json:"price,omitempty"`
	Agent      string    `json:"agent,omitempty"`
	Strategy   string    `json:"strategy,omitempty"`
	Confidence float64   `json:"confidence,omitempty"`
	Timestamp  time.Time `json:"timestamp,omitempty"`
}

func (o Order) EffectiveID() string {
	if o.ID != "" {
		return o.ID
	}
	return o.OrderID
}

type Fill struct {
	FillID     string    `json:"fillId,omitempty"`
	OrderID    string    `json:"orderId"`
	Symbol     string    `json:"symbol"`
	Side       string    `json:"side"`
	Qty        float64   `json:"qty"`
	Price      float64   `json:"price"`
	Commission float64   `json:"commission,omitempty"`
	Agent      string    `json:"agent,omitempty"`
	Strategy   string    `json:"strategy,omitempty"`
	Timestamp  time.Time `json:"timestamp,omitempty"`
}

type OrderRejection struct {
	OrderID   string `json:"orderId"`
	Reason    string `json:"reason"`
	Timestamp int64  `json:"timestamp"`
}

type PortfolioState struct {
	Equity     float64   `json:"equity"`
	DailyLoss  float64   `json:"dailyLoss"`
	PeakEquity float64   `json:"peakEquity"`
	Drawdown   float64   `json:"drawdown"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
