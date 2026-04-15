package brokers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/omega-prime/omega-prime-delta/backend/internal/models"
)

// Order models a normalized execution request.
type Order struct {
	ID       string
	Symbol   string
	Side     string
	Type     string
	Quantity float64
	Price    float64
	Broker   string
	Metadata map[string]any
}

// Fill models an execution response from a broker.
type Fill struct {
	OrderID      string
	Broker       string
	Status       string
	FilledQty    float64
	AveragePrice float64
	ExternalID   string
	ReceivedAt   time.Time
}

// Adapter defines the broker API used by execution services.
type Adapter interface {
	Name() string
	ValidateConfig() error
	SubmitOrder(ctx context.Context, order Order) (Fill, error)
	Close(ctx context.Context) error
}

// Config is the shared broker adapter configuration.
type Config struct {
	RequestTimeout time.Duration
	DefaultLatency time.Duration
}

// ConfigFromEnv reads mandatory broker config from the process environment.
func ConfigFromEnv() (Config, error) {
	requestTimeout := envDuration("BROKER_REQUEST_TIMEOUT", 5*time.Second)
	defaultLatency := envDuration("BROKER_DEFAULT_LATENCY", 75*time.Millisecond)

	if requestTimeout <= 0 {
		return Config{}, errors.New("BROKER_REQUEST_TIMEOUT must be > 0")
	}
	if defaultLatency < 0 {
		return Config{}, errors.New("BROKER_DEFAULT_LATENCY must be >= 0")
	}

	return Config{RequestTimeout: requestTimeout, DefaultLatency: defaultLatency}, nil
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	if numeric, err := strconv.Atoi(value); err == nil {
		return time.Duration(numeric) * time.Millisecond
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

type simulatedAdapter struct {
	name   string
	config Config
	logger *slog.Logger
}

func newSimulatedAdapter(name string, config Config, logger *slog.Logger) Adapter {
	return &simulatedAdapter{name: name, config: config, logger: logger}
}

func (a *simulatedAdapter) Name() string {
	return a.name
}

func (a *simulatedAdapter) ValidateConfig() error {
	if a.name == "" {
		return errors.New("adapter name cannot be empty")
	}
	if a.logger == nil {
		return errors.New("adapter logger cannot be nil")
	}
	if a.config.RequestTimeout <= 0 {
		return errors.New("request timeout must be > 0")
	}
	return nil
}

func (a *simulatedAdapter) SubmitOrder(ctx context.Context, order Order) (Fill, error) {
	if err := a.ValidateConfig(); err != nil {
		return Fill{}, err
	}
	if order.ID == "" {
		return Fill{}, errors.New("order id cannot be empty")
	}
	if order.Quantity <= 0 {
		return Fill{}, errors.New("quantity must be > 0")
	}

	delay := a.config.DefaultLatency
	if delay <= 0 {
		delay = 25 * time.Millisecond
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return Fill{}, ctx.Err()
	case <-timer.C:
		fill := Fill{
			OrderID:      order.ID,
			Broker:       a.name,
			Status:       "filled",
			FilledQty:    order.Quantity,
			AveragePrice: order.Price,
			ExternalID:   fmt.Sprintf("%s-%s", a.name, order.ID),
			ReceivedAt:   time.Now().UTC(),
		}
		a.logger.Info("broker fill generated",
			"broker", a.name,
			"order_id", order.ID,
			"symbol", order.Symbol,
			"qty", order.Quantity,
		)
		return fill, nil
	}
}

func (a *simulatedAdapter) Close(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		a.logger.Info("broker adapter closed", "broker", a.name)
		return nil
	}
}

// ExecutionBroker defines a normalized broker interface for order execution.
type ExecutionBroker interface {
	Submit(order models.Order) (*models.Fill, error)
}

// PaperBroker simulates market execution with simple slippage.
type PaperBroker struct{}

func NewPaperBroker() *PaperBroker {
	return &PaperBroker{}
}

func (b *PaperBroker) Submit(order models.Order) (*models.Fill, error) {
	time.Sleep(20 * time.Millisecond)
	fillPrice := order.Price
	if order.Side == "BUY" {
		fillPrice *= 1.0001
	} else {
		fillPrice *= 0.9999
	}
	orderID := order.EffectiveID()
	return &models.Fill{
		FillID:    uuid.New().String(),
		OrderID:   orderID,
		Symbol:    order.Symbol,
		Side:      order.Side,
		Qty:       order.Qty,
		Price:     fillPrice,
		Timestamp: time.Now().UTC(),
		Agent:     order.Agent,
		Strategy:  order.Strategy,
	}, nil
}
