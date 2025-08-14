package eventstore

import (
	"context"
	"errors"

	"github.com/salesworks/s-works/api/internal/platform/messaging"
)

var (
	// ErrConcurrencyConflict is returned when an event with the same aggregate version already exists.
	ErrConcurrencyConflict = errors.New("concurrency conflict: event version already exists for this aggregate")
)

// Store is the interface for saving and retrieving events.
type Store interface {
	// Save saves one or more event envelopes to the store.
	Save(ctx context.Context, envelopes ...*messaging.EventEnvelope) error
}
