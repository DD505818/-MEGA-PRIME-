package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type config struct {
	Port              string
	DefaultCash       float64
	BaseCurrency      string
	ReconcileInterval time.Duration
}

type portfolioState struct {
	mu           sync.RWMutex
	cash         float64
	baseCurrency string
	updatedAt    time.Time
}

func (s *portfolioState) snapshot() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]any{
		"cash":          s.cash,
		"base_currency": s.baseCurrency,
		"updated_at":    s.updatedAt.Format(time.RFC3339),
	}
}

func loadConfig() (config, error) {
	port := envOrDefault("PORTFOLIO_SERVICE_PORT", "3001")
	if _, err := strconv.Atoi(port); err != nil {
		return config{}, errors.New("PORTFOLIO_SERVICE_PORT must be numeric")
	}

	defaultCash := 1_000_000.0
	if raw := strings.TrimSpace(os.Getenv("PORTFOLIO_DEFAULT_CASH")); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || parsed <= 0 {
			return config{}, errors.New("PORTFOLIO_DEFAULT_CASH must be a positive float")
		}
		defaultCash = parsed
	}

	reconcileEvery := envDurationOrDefault("PORTFOLIO_RECONCILE_INTERVAL", 30*time.Second)
	if reconcileEvery <= 0 {
		return config{}, errors.New("PORTFOLIO_RECONCILE_INTERVAL must be > 0")
	}

	baseCurrency := strings.ToUpper(strings.TrimSpace(os.Getenv("PORTFOLIO_BASE_CURRENCY")))
	if baseCurrency == "" {
		return config{}, errors.New("PORTFOLIO_BASE_CURRENCY is required")
	}

	return config{Port: port, DefaultCash: defaultCash, BaseCurrency: baseCurrency, ReconcileInterval: reconcileEvery}, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := loadConfig()
	if err != nil {
		logger.Error("invalid startup configuration", "error", err)
		os.Exit(1)
	}

	state := &portfolioState{cash: cfg.DefaultCash, baseCurrency: cfg.BaseCurrency, updatedAt: time.Now().UTC()}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	reconcileTicker := time.NewTicker(cfg.ReconcileInterval)
	defer reconcileTicker.Stop()
	go func() {
		logger.Info("portfolio reconciler started", "interval", cfg.ReconcileInterval.String())
		for {
			select {
			case <-ctx.Done():
				logger.Info("portfolio reconciler stopping")
				return
			case <-reconcileTicker.C:
				state.mu.Lock()
				state.updatedAt = time.Now().UTC()
				state.mu.Unlock()
				logger.Info("portfolio reconciliation completed")
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/portfolio", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state.snapshot())
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("portfolio service started", "port", cfg.Port, "base_currency", cfg.BaseCurrency)
		if serveErr := srv.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Error("http server failed", "error", serveErr)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("portfolio service stopped")
}
