package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type config struct {
	Port              string
	IngestionInterval time.Duration
	MarketSource      string
}

func loadConfig() (config, error) {
	cfg := config{
		Port:              envOrDefault("MARKET_INGESTION_PORT", "3005"),
		MarketSource:      strings.TrimSpace(os.Getenv("MARKET_SOURCE")),
		IngestionInterval: envDurationOrDefault("INGESTION_INTERVAL", 5*time.Second),
	}

	if cfg.MarketSource == "" {
		return config{}, errors.New("MARKET_SOURCE is required")
	}
	if _, err := strconv.Atoi(cfg.Port); err != nil {
		return config{}, errors.New("MARKET_INGESTION_PORT must be numeric")
	}
	if cfg.IngestionInterval <= 0 {
		return config{}, errors.New("INGESTION_INTERVAL must be > 0")
	}
	return cfg, nil
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ticker := time.NewTicker(cfg.IngestionInterval)
	defer ticker.Stop()

	go func() {
		logger.Info("market ingestion loop started", "source", cfg.MarketSource, "interval", cfg.IngestionInterval.String())
		for {
			select {
			case <-ctx.Done():
				logger.Info("market ingestion loop stopping")
				return
			case <-ticker.C:
				logger.Info("ingestion tick", "source", cfg.MarketSource)
			}
		}
	}()

	go func() {
		logger.Info("market ingestion service started", "port", cfg.Port)
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Error("http server failed", "error", serveErr)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("market ingestion service stopped")
}
