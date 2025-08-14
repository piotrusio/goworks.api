package messaging

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// EventEnvelope wraps domain events with metadata
type EventEnvelope struct {
	EventID          string      `json:"event_id"`
	EventType        string      `json:"event_type"`
	AggregateID      string      `json:"aggregate_id"`
	AggregateType    string      `json:"aggregate_type"`
	AggregateVersion int         `json:"aggregate_version"`
	EventVersion     int         `json:"event_version"`
	Timestamp        time.Time   `json:"timestamp"`
	CorrelationID    string      `json:"correlation_id,omitempty"`
	CausationID      string      `json:"causation_id,omitempty"`
	UserID           string      `json:"user_id,omitempty"`
	Payload          interface{} `json:"payload"`
}

// EnvelopeOption is a functional option for configuring EventEnvelope
type EnvelopeOption func(*EventEnvelope)

// WithCorrelationID sets the correlation ID for request tracing
func WithCorrelationID(id string) EnvelopeOption {
	return func(e *EventEnvelope) {
		e.CorrelationID = id
	}
}

// WithCausationID sets the causation ID to track event chains
func WithCausationID(id string) EnvelopeOption {
	return func(e *EventEnvelope) {
		e.CausationID = id
	}
}

// WithUserID sets the user ID for audit purposes
func WithUserID(userID string) EnvelopeOption {
	return func(e *EventEnvelope) {
		e.UserID = userID
	}
}

func NewEventEnvelope(
	eventType, aggregateID, aggregateType string,
	aggregateVersion int,
	payload interface{},
	options ...EnvelopeOption,
) *EventEnvelope {
	envelope := &EventEnvelope{
		EventID:          uuid.New().String(),
		EventType:        eventType,
		EventVersion:     1,
		AggregateID:      aggregateID,
		AggregateType:    aggregateType,
		AggregateVersion: aggregateVersion,
		Timestamp:        time.Now(),
		Payload:          payload,
	}

	// Apply optional configuration
	for _, option := range options {
		option(envelope)
	}

	return envelope
}

// Validate checks if the envelope has all required fields
func (e *EventEnvelope) Validate() error {
	if e.EventType == "" {
		return errors.New("event type is required")
	}
	if e.AggregateID == "" {
		return errors.New("aggregate ID is required")
	}
	if e.AggregateType == "" {
		return errors.New("aggregate type is required")
	}
	if e.Payload == nil {
		return errors.New("payload is required")
	}
	return nil
}
