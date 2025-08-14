package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"

	"github.com/salesworks/s-works/api/internal/fabrics/domain"
	command "github.com/salesworks/s-works/api/internal/platform/context"
	"github.com/salesworks/s-works/api/internal/platform/messaging"
	"github.com/salesworks/s-works/api/internal/platform/validator"
)

// FabricEventHandler contains the business logic for processing ERP events for fabrics.
// It implements the subscriber.MessageHandler interface.
type FabricEventHandler struct {
	service FabricCommandService
	logger  *slog.Logger
}

type erpFabricEvent struct {
	Code        string `json:"fabric_code"`
	Name        string `json:"fabric_name"`
	MeasureUnit string `json:"measure_unit,omitempty"`
	OfferStatus string `json:"offer_status,omitempty"`
}

func NewFabricEventHandler(service FabricCommandService, logger *slog.Logger) *FabricEventHandler {
	return &FabricEventHandler{
		service: service,
		logger:  logger.With("component", "erpEventHandler"),
	}
}

// HandleMessage is the entry point called by the NatsSubscriber.
func (h *FabricEventHandler) HandleMessage(ctx context.Context, subject string, payload []byte) error {
	// Deserialize the payload into an EventEnvelope first.
	var envelope messaging.EventEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		h.logger.Error("Failed to unmarshal event envelope", "error", err, "subject", subject)
		// Return nil to prevent retries for malformed messages.
		return nil
	}

	// Validate the envelope
	if err := envelope.Validate(); err != nil {
		h.logger.Error("Invalid event envelope", "error", err, "subject", subject)
		return nil
	}

	return h.adaptEventToCommand(ctx, envelope)
}

func (h *FabricEventHandler) adaptEventToCommand(ctx context.Context, envelope messaging.EventEnvelope) error {
	// Extract payload from envelope
	payloadBytes, err := json.Marshal(envelope.Payload)
	if err != nil {
		h.logger.Error("Failed to marshal payload", "error", err, "event_id", envelope.EventID)
		return nil
	}

	var erpEvent erpFabricEvent
	if err := json.Unmarshal(payloadBytes, &erpEvent); err != nil {
		h.logger.Error("Failed to unmarshal payload to erpFabricEvent", "error", err, "event_id", envelope.EventID)
		return nil
	}

	switch envelope.EventType {
	case "erp.fabric.created":
		return h.handleCreateEvent(ctx, erpEvent, envelope.EventID)
	case "erp.fabric.updated":
		return h.handleUpdateEvent(ctx, erpEvent, envelope.EventID, envelope.AggregateVersion)
	case "erp.fabric.deleted":
		return h.handleDeleteEvent(ctx, erpEvent, envelope.EventID, envelope.AggregateVersion)
	default:
		h.logger.Warn("Received unknown ERP event, discarding", "type", envelope.EventType)
		return nil
	}
}

func (h *FabricEventHandler) handleCreateEvent(
	ctx context.Context, event erpFabricEvent, eventID string,
) error {

	ctx = command.WithCommandSource(ctx, command.CommandSourceEvent)
	event = h.withDefaults(event)

	v := validator.New()
	validateCreateFabricEvent(v, event)
	if !v.Valid() {
		h.logger.Error(
			"Invalid fabric data from ERP event",
			"errors", v.Errors, "code", event.Code, "event_id", eventID,
		)
		return nil // Don't retry validation errors
	}

	_, err := h.service.CreateFabric(
		ctx,
		event.Code,        // code
		event.Name,        // name
		event.MeasureUnit, // measureUnit (default if not provided)
		event.OfferStatus, // offerStatus (default if not provided)
	)

	if err != nil {
		// Handle domain errors the same way as REST handler
		switch {
		case errors.Is(err, domain.ErrDuplicateFabricCode):
			h.logger.Info("Fabric already exists, skipping", "code", event.Code, "event_id", eventID)
			return nil // Idempotent - don't error on duplicates from events
		case errors.Is(err, domain.ErrInvalidFabricCodeLength) ||
			errors.Is(err, domain.ErrInvalidFabricCodePattern) ||
			errors.Is(err, domain.ErrInvalidFabricNameLength):
			h.logger.Error("Invalid fabric data from ERP", "error", err, "code", event.Code, "event_id", eventID)
			return nil // Don't retry validation errors
		default:
			h.logger.Error("Failed to create fabric", "error", err, "code", event.Code, "event_id", eventID)
			return err // Retry infrastructure errors
		}
	}

	h.logger.Info("Fabric created from event", "code", event.Code, "event_id", eventID)
	return nil
}

func (h *FabricEventHandler) handleUpdateEvent(
	ctx context.Context, event erpFabricEvent, eventID string, version int,
) error {
	ctx = command.WithCommandSource(ctx, command.CommandSourceEvent)

	event = h.withDefaults(event)

	v := validator.New()
	validateUpdateFabricEvent(v, version, event)
	if !v.Valid() {
		h.logger.Error(
			"Invalid fabric data from ERP event",
			"errors", v.Errors, "code", event.Code, "event_id", eventID,
		)
		return nil // Don't retry validation errors
	}

	_, err := h.service.UpdateFabric(
		ctx,
		event.Code,        // code
		event.Name,        // name
		event.MeasureUnit, // measureUnit
		event.OfferStatus, // offerStatus
		version-1,         // version sent by the erp system is the next version,
		// to keep it consistent with the REST API we need to subtract 1
	)

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrRecordNotFound):
			h.logger.Warn(
				"Fabric not found for update, might need to create first",
				"code", event.Code, "event_id", eventID,
			)
			return nil
		case errors.Is(err, domain.ErrConcurrencyConflict):
			h.logger.Warn(
				"Version conflict, event might be out of order",
				"code", event.Code, "version", version, "event_id", eventID,
			)
			return nil // Don't retry version conflicts from events
		case errors.Is(err, domain.ErrInvalidFabricNameLength):
			h.logger.Error(
				"Invalid fabric data from ERP",
				"error", err, "code", event.Code, "event_id", eventID,
			)
			return nil
		default:
			h.logger.Error(
				"Failed to update fabric",
				"error", err, "code", event.Code, "event_id", eventID,
			)
			return err
		}
	}

	h.logger.Info(
		"Fabric updated from event",
		"code", event.Code, "version", version, "event_id", eventID,
	)
	return nil
}

func (h *FabricEventHandler) handleDeleteEvent(
	ctx context.Context, event erpFabricEvent, eventID string, version int,
) error {
	ctx = command.WithCommandSource(ctx, command.CommandSourceEvent)

	err := h.service.DeleteFabric(ctx, event.Code, version)

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrRecordNotFound):
			h.logger.Info(
				"Fabric already deleted or not found",
				"code", event.Code, "event_id", eventID,
			)
			return nil // Idempotent
		case errors.Is(err, domain.ErrConcurrencyConflict):
			h.logger.Warn(
				"Version conflict on delete",
				"code", event.Code, "version", version, "event_id", eventID,
			)
			return nil
		default:
			h.logger.Error(
				"Failed to delete fabric",
				"error", err, "code", event.Code, "event_id", eventID,
			)
			return err
		}
	}

	h.logger.Info(
		"Fabric deleted from event",
		"code", event.Code, "version", version, "event_id", eventID,
	)
	return nil
}

func (h *FabricEventHandler) withDefaults(event erpFabricEvent) erpFabricEvent {
	if event.MeasureUnit == "" {
		event.MeasureUnit = "MB" // or whatever your default is
	}
	if event.OfferStatus == "" {
		event.OfferStatus = "ACTIVE" // or whatever your default is
	}
	return event
}

func validateCreateFabricEvent(v *validator.Validator, event erpFabricEvent) {
	// --- Fabric Code Validation ---
	v.Check(event.Code != "", "code", "code must be provided")
	v.Check(len(event.Code) >= 2, "code", "code must be between 2 and 30 characters long")
	v.Check(len(event.Code) <= 30, "code", "code must be between 2 and 30 characters long")
	v.Check(validator.Matches(
		event.Code, regexp.MustCompile("^[A-Z0-9]+$")),
		"code", "code must only contain uppercase letters and numbers",
	)

	// --- Fabric Name Validation ---
	v.Check(event.Name != "", "name", "name must be provided")
	v.Check(len(event.Name) <= 250, "name", "name must not be more than 250 characters long")
}

func validateUpdateFabricEvent(v *validator.Validator, version int, event erpFabricEvent) {
	v.Check(version > 0, "version", "version must be provided and greater than 0")
	v.Check(event.Name != "", "name", "name must be provided")
	v.Check(len(event.Name) <= 250, "name", "name must not be more than 250 characters long")
}
