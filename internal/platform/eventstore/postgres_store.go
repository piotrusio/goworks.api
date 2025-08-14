package eventstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/salesworks/s-works/api/internal/platform/messaging"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{
		db: db,
	}
}

func (s *PostgresStore) Save(ctx context.Context, envelopes ...*messaging.EventEnvelope) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("could not begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO events (
			event_id, aggregate_id, aggregate_type, event_type,
			aggregate_version, payload, "timestamp", correlation_id, user_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`)
	if err != nil {
		return fmt.Errorf("could not prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, envelope := range envelopes {
		_, err := stmt.ExecContext(ctx,
			envelope.EventID,
			envelope.AggregateID,
			envelope.AggregateType,
			envelope.EventType,
			envelope.AggregateVersion,
			envelope.Payload,
			envelope.Timestamp,
			envelope.CorrelationID,
			envelope.UserID,
		)

		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrConcurrencyConflict
			}
			return fmt.Errorf("could not execute statement for event %s: %w", envelope.EventID, err)
		}
	}

	return tx.Commit()
}
