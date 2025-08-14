package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
)

// Publisher interface that can be used by domain services
type Publisher interface {
	Publish(ctx context.Context, subject string, envelope *EventEnvelope) error
	Close() error
}

// EventPublisher is a generic publisher for all domain events
type NatsPublisher struct {
	conn   *nats.Conn
	logger *slog.Logger
}

// NewEventPublisher creates a new generic event publisher
func NewNatsPublisher(conn *nats.Conn, logger *slog.Logger) *NatsPublisher {
	return &NatsPublisher{
		conn:   conn,
		logger: logger.With("component", "NatsPublisher"),
	}
}

// Publish publishes an event envelope to the topic
func (p *NatsPublisher) Publish(ctx context.Context, subject string, envelope *EventEnvelope) error {
	// Validate the envelope
	if err := envelope.Validate(); err != nil {
		return fmt.Errorf("invalid event envelope: %w", err)
	}

	// Serialize the envelope to JSON
	event, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal event envelope: %w", err)
	}

	if err := p.conn.Publish(subject, event); err != nil {
		return fmt.Errorf("failed to publish message to subject '%s': %w", subject, err)
	}

	p.logger.Debug(
		"Message published to NATS",
		"subject", subject,
		"aggregate_type", envelope.AggregateType,
	)

	return nil
}

func (p *NatsPublisher) Close() error {
	if p.conn != nil && !p.conn.IsClosed() {
		p.logger.Info("Draining and closing NATS connection.")
		// Drain ensures all buffered messages are sent before closing.
		return p.conn.Drain()
	}
	return nil
}
