package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/salesworks/s-works/api/internal/bootstrap"
	"github.com/salesworks/s-works/api/internal/platform/database"
	"github.com/salesworks/s-works/api/internal/platform/httpx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
)

const version = "1.0.0"

type postgresConfig struct {
	uri          string
	maxOpenConns int
	maxIdleConns int
	maxIdleTime  time.Duration
}

type clerkConfig struct {
	secretKey string
}

type natsConfig struct {
	url string
}

type config struct {
	port     int
	env      string
	clerk    clerkConfig
	postgres postgresConfig
	nats     natsConfig
}

type api struct {
	config       config
	logger       *slog.Logger
	services     bootstrap.Services
	repositories bootstrap.Repositories
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	setupOtelPropagator()
	cfg := loadConfig()

	logger := newLogger(cfg.env)
	logger = logger.With("env", cfg.env, "component", "api")

	appCtx, stop := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer stop()

	startupCtx, startupCancel := context.WithTimeout(appCtx, 30*time.Second)
	defer startupCancel()

	dbCtx := httpx.WithLogger(startupCtx, logger)
	postgres, err := database.NewPostgresDB(
		dbCtx,
		cfg.postgres.uri,
		cfg.postgres.maxOpenConns,
		cfg.postgres.maxIdleConns,
		cfg.postgres.maxIdleTime,
		logger,
	)
	if err != nil {
		logger.Error("failed to initialized postgres database", "error", err)
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer func() {
		postgres.Close()
		logger.Info("postgres database connection pool closed")
	}()
	logger.Info("succesfully connected to postgres database")

	natsConn, err := nats.Connect(cfg.nats.url)
	if err != nil {
		logger.Error("failed to connect to NATS", "error", err)
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	defer natsConn.Close()
	logger.Info("successfully connected to NATS server")

	repositories := bootstrap.NewRepositories(postgres)
	services := bootstrap.NewServices(repositories, natsConn, logger)

	if _, err := setupMetrics(); err != nil {
		logger.Error("failed to setup metrics", "error", err)
		return fmt.Errorf("failed to initialize metrics: %w", err)
	}

	api := &api{
		config:       cfg,
		logger:       logger,
		services:     services,
		repositories: repositories,
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.port),
		Handler:      api.routes(promhttp.Handler()),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	subscribers := NewSubscribers(natsConn, services, logger)
	go subscribers.Start()

	go func() {
		logger.Info("starting server", "addr", srv.Addr)
		if errSrv := srv.ListenAndServe(); errSrv != nil && errSrv != http.ErrServerClosed {
			logger.Error("HTTP server ListenAndServe error", "error", errSrv)
			stop()
		}
	}()

	<-appCtx.Done()
	logger.Info("shutdown initiated", "signal", "termination")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	var shutdownErr error

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
		shutdownErr = err
	} else {
		logger.Info("HTTP server gracefully stopped.")
	}

	logger.Info("service exiting.")
	return shutdownErr
}

func loadConfig() config {
	var cfg config

	cfg.nats.url = os.Getenv("NATS_URL")
	if cfg.nats.url == "" {
		panic("NATS_URL environment variable must be set")
	}

	cfg.postgres.uri = os.Getenv("POSTGRES_URI")
	if cfg.postgres.uri == "" {
		panic("POSTGRES_URI environment variable must be set")
	}

	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8080"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		panic(fmt.Sprintf("invalid PORT env var: %v", err))
	}
	cfg.port = port

	cfg.env = os.Getenv("ENV")
	if cfg.env == "" {
		cfg.env = "development"
	}

	openConns := os.Getenv("POSTGRES_OPEN_CONNS")
	if openConns == "" {
		openConns = "25"
	}
	maxOpenConns, err := strconv.Atoi(openConns)
	if err != nil {
		panic(fmt.Sprintf("invalid POSTGRES_OPEN_CONNS env var: %v", err))
	}
	cfg.postgres.maxOpenConns = maxOpenConns

	idleConns := os.Getenv("POSTGRES_IDLE_CONNS")
	if idleConns == "" {
		idleConns = "25"
	}
	maxIdleConns, err := strconv.Atoi(idleConns)
	if err != nil {
		panic(fmt.Sprintf("invalid POSTGRES_IDLE_CONNS env var: %v", err))
	}
	cfg.postgres.maxIdleConns = maxIdleConns

	idleTime := os.Getenv("POSTGRES_IDLE_TIME")
	if idleTime == "" {
		idleTime = "15m"
	}
	maxIdleTime, err := time.ParseDuration(idleTime)
	if err != nil {
		panic(fmt.Sprintf("invalid POSTGRES_IDLE_TIME env var: %v", err))
	}
	cfg.postgres.maxIdleTime = maxIdleTime
	return cfg
}

func newLogger(env string) *slog.Logger {
	var handler slog.Handler
	if env == "development" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	return slog.New(handler)
}

func setupMetrics() (*prometheus.Exporter, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("create prometheus exporter: %w", err)
	}

	meterProvider := metric.NewMeterProvider(metric.WithReader(exporter))
	otel.SetMeterProvider(meterProvider)

	return exporter, nil
}

// global propagator for OpenTelemetry.
func setupOtelPropagator() {
	// NewCompositeTextMapPropagator allows OTel to understand multiple header formats.
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, // W3C TraceContext format (standard)
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)
}
