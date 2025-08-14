package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	// Switch to pgx driver for PostgreSQL
	_ "github.com/jackc/pgx/v5/stdlib"
)

// DB manages the database connection pool and related dependencies.
type PostgresDB struct {
	Pool   *sql.DB
	logger *slog.Logger
}

// New initializes the database connection pool using the provided uri string
// and logger. It wraps the pool in a DB struct.
// Accepts uri string, maxOpen, maxIdle, maxIdleTime directly.
func NewPostgresDB(
	ctx context.Context,
	uri string,
	maxOpenConns int,
	maxIdleConns int,
	maxIdleTime time.Duration,
	logger *slog.Logger,
) (*PostgresDB, error) {

	if uri == "" {
		return nil, fmt.Errorf("database uri string is empty")
	}

	// Use pgx driver
	pool, err := sql.Open("pgx", uri)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Set pool parameters from arguments
	pool.SetMaxOpenConns(maxOpenConns)
	pool.SetMaxIdleConns(maxIdleConns)
	pool.SetConnMaxIdleTime(maxIdleTime)

	// Verify the connection with a ping and timeout (use startup context)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = pool.PingContext(pingCtx)
	if err != nil {
		// Ensure pool is closed if ping fails to prevent leaks
		if closeErr := pool.Close(); closeErr != nil {
			logger.Error("Failed to close pool after ping error",
				"closeError", closeErr)
		}
		logger.Error("Database ping failed", "error", err)
		// Don't wrap the original driver error directly in production
		// logs to avoid leaking details.
		return nil, fmt.Errorf("unable to verify database connection")
	}

	logger.Info("Database connection pool established",
		"maxOpenConns", maxOpenConns,
		"maxIdleConns", maxIdleConns,
		"maxIdleTime", maxIdleTime,
	)

	// Return the wrapper struct containing the pool and logger
	return &PostgresDB{Pool: pool, logger: logger}, nil
}

// Close gracefully closes the database connection pool.
func (db *PostgresDB) Close() {
	if db.Pool != nil {
		db.logger.Info("Closing database connection pool.")
		// sql.DB.Close() waits for connections to be returned before closing.
		if err := db.Pool.Close(); err != nil {
			db.logger.Error("Error closing database connection pool",
				"error", err)
		}
	}
}
