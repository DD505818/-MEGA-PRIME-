package execution

import (
	"github.com/google/uuid"
	"github.com/omega-prime/omega-prime-delta/backend/internal/brokers"
	"github.com/omega-prime/omega-prime-delta/backend/internal/models"
)

type Router struct {
	paper *brokers.PaperBroker
}

func NewRouter() *Router {
	return &Router{paper: brokers.NewPaperBroker()}
}

func (r *Router) Execute(order models.Order) (*models.Fill, error) {
	fill, err := r.paper.Submit(order)
	if err != nil {
		return nil, err
	}
	if fill.FillID == "" {
		fill.FillID = uuid.NewString()
	}
	return fill, nil
}
