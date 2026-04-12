package risk

import "errors"

var (
	ErrInvalidOrder  = errors.New("invalid order")
	ErrRejectedOrder = errors.New("order rejected by risk policy")
)

type Order struct {
	Symbol string
	Side   string
	Qty    float64
	Price  float64
}

type Limits struct {
	MaxOrderQty   float64
	MaxOrderNotional float64
}

func Validate(order Order, limits Limits) error {
	if order.Symbol == "" || order.Side == "" || order.Qty <= 0 || order.Price <= 0 {
		return ErrInvalidOrder
	}
	if limits.MaxOrderQty > 0 && order.Qty > limits.MaxOrderQty {
		return ErrRejectedOrder
	}
	notional := order.Qty * order.Price
	if limits.MaxOrderNotional > 0 && notional > limits.MaxOrderNotional {
		return ErrRejectedOrder
	}
	return nil
}
