package execution

import (
	"errors"

	"github.com/omega-prime/omega-prime-delta/backend/internal/brokers"
	"github.com/omega-prime/omega-prime-delta/backend/internal/models"
)

type Router struct {
	broker brokers.ExecutionBroker
}

func NewRouter() *Router {
	return &Router{broker: brokers.NewPaperBroker()}
}

func (r *Router) Execute(order models.Order) (*models.Fill, error) {
	if r == nil || r.broker == nil {
		return nil, errors.New("execution router not configured")
	}
	return r.broker.Submit(order)
}
