package messaging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEventEnvelope_CreatesEnvelopeWithRequiredFields(t *testing.T) {
	// Arrange
	eventType := "fabric.created"
	aggregateID := "FABRIC001"
	aggregateType := "fabric"
	aggregateVersion := 1
	payload := map[string]interface{}{
		"code": "FABRIC001",
		"name": "Test Fabric",
	}

	// Act
	envelope := NewEventEnvelope(eventType, aggregateID, aggregateType, aggregateVersion, payload)

	// Assert
	assert.Equal(t, eventType, envelope.EventType)
	assert.Equal(t, aggregateID, envelope.AggregateID)
	assert.Equal(t, aggregateType, envelope.AggregateType)
	assert.Equal(t, aggregateVersion, envelope.AggregateVersion)
	assert.Equal(t, 1, envelope.EventVersion)
	assert.NotEmpty(t, envelope.EventID)
	assert.False(t, envelope.Timestamp.IsZero())
	assert.NotNil(t, envelope.Payload)
}

func TestEventEnvelope_WithOptionalMetadata(t *testing.T) {
	// Arrange
	payload := map[string]interface{}{"test": "data"}

	// Act
	envelope := NewEventEnvelope(
		"fabric.created",
		"FABRIC01",
		"fabric",
		1,
		payload,
		WithCorrelationID("correlation-123"),
		WithCausationID("causation-456"),
		WithUserID("user-789"),
	)

	assert.Equal(t, "correlation-123", envelope.CorrelationID)
	assert.Equal(t, "causation-456", envelope.CausationID)
	assert.Equal(t, "user-789", envelope.UserID)
}

func TestEventEnvelope_Validation(t *testing.T) {
	tests := []struct {
		name        string
		envelope    *EventEnvelope
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid envelope",
			envelope: NewEventEnvelope(
				"fabric.created",
				"FABRIC001",
				"fabric",
				1,
				map[string]interface{}{"test": "data"},
			),
			expectError: false,
		},
		{
			name: "empty event type",
			envelope: &EventEnvelope{
				EventID:          "test-id",
				EventType:        "",
				AggregateID:      "FABRIC001",
				AggregateType:    "fabric",
				AggregateVersion: 1,
				Payload:          map[string]interface{}{"test": "data"},
			},
			expectError: true,
			errorMsg:    "event type is required",
		},
		{
			name: "empty aggregate ID",
			envelope: &EventEnvelope{
				EventID:          "test-id",
				EventType:        "fabric.created",
				AggregateID:      "",
				AggregateType:    "fabric",
				AggregateVersion: 1,
				Payload:          map[string]interface{}{"test": "data"},
			},
			expectError: true,
			errorMsg:    "aggregate ID is required",
		},
		{
			name: "nil payload",
			envelope: &EventEnvelope{
				EventID:          "test-id",
				EventType:        "fabric.created",
				AggregateID:      "FABRIC001",
				AggregateType:    "fabric",
				AggregateVersion: 1,
				Payload:          nil,
			},
			expectError: true,
			errorMsg:    "payload is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.envelope.Validate()
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
