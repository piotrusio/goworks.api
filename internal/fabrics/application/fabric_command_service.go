package application

import (
	"context"
	"fmt"

	"github.com/salesworks/s-works/api/internal/fabrics/domain"
	command "github.com/salesworks/s-works/api/internal/platform/context"
	"github.com/salesworks/s-works/api/internal/platform/eventstore"
	"github.com/salesworks/s-works/api/internal/platform/httpx"
	"github.com/salesworks/s-works/api/internal/platform/messaging"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

type FabricService struct {
	commandRepo  domain.FabricCommandRepository
	publisher    messaging.Publisher
	eventStore   eventstore.Store
	eventChannel string
}

func NewFabricCommandService(
	commandRepo domain.FabricCommandRepository,
	publisher messaging.Publisher,
	eventStore eventstore.Store,
) *FabricService {
	return &FabricService{
		commandRepo:  commandRepo,
		publisher:    publisher,
		eventStore:   eventStore,
		eventChannel: "app.fabric",
	}
}

func (s *FabricService) CreateFabric(
	ctx context.Context, code, name, measureUnit, offerStatus string,
) (*domain.Fabric, error) {
	ctx, span := otel.Tracer("s-works/api").Start(ctx, "fabric.service.create")
	defer span.End()
	logger := httpx.GetLogger(ctx).With("component", "fabric.service")

	fabric, err := domain.NewFabric(code, name, measureUnit, offerStatus)
	if err != nil {
		wrappedErr := fmt.Errorf("application service failed to create fabric: %w", err)
		logger.Error("fabric creation failed due to a domain error", "error", wrappedErr)
		span.RecordError(wrappedErr)
		span.SetStatus(codes.Error, "domain rule violation")
		return nil, wrappedErr
	}

	persistedFabric, err := s.commandRepo.Save(ctx, fabric)
	if err != nil {
		wrappedErr := fmt.Errorf("failed to save fabric: %w", err)
		logger.Error("saving fabric failed", "error", wrappedErr)
		span.RecordError(wrappedErr)
		span.SetStatus(codes.Error, "database write error")
		return nil, wrappedErr
	}

	var envelopesToPublish []*messaging.EventEnvelope
	for _, event := range persistedFabric.Events() {
		var eventType string
		switch event.(type) {
		case domain.FabricCreated:
			eventType = "app.fabric.created"
		case domain.FabricReactivated:
			eventType = "app.fabric.reactivated"
		default:
			continue
		}

		envelope := messaging.NewEventEnvelope(
			eventType,
			persistedFabric.Code,
			"fabric",
			persistedFabric.Version,
			event,
		)
		envelopesToPublish = append(envelopesToPublish, envelope)
	}

	if len(envelopesToPublish) > 0 {
		if err := s.eventStore.Save(ctx, envelopesToPublish...); err != nil {
			wrappedErr := fmt.Errorf("failed to save to event store: %w", err)
			logger.Error("saving to event store failed", "error", wrappedErr)
			span.RecordError(wrappedErr)
			span.SetStatus(codes.Error, "event store write error")
			return nil, wrappedErr
		}

		// the contextet may be from REST API or from NATS subscription
		if command.IsFromREST(ctx) {
			for _, envelope := range envelopesToPublish {
				if err := s.publisher.Publish(ctx, s.eventChannel, envelope); err != nil {
					wrappedErr := fmt.Errorf("failed to publish fabric event envelope: %w", err)
					logger.Error(
						"publishing event envelope failed",
						"error", wrappedErr, "eventID", envelope.EventID,
					)
					span.RecordError(wrappedErr)
				}
			}
		}
	}

	return persistedFabric, nil
}

func (s *FabricService) UpdateFabric(
	ctx context.Context, code, name, measureUnit, offerStatus string, version int,
) (*domain.Fabric, error) {
	ctx, span := otel.Tracer("s-works/api").Start(ctx, "fabric.service.update")
	defer span.End()
	logger := httpx.GetLogger(ctx).With("component", "fabric.service")

	fabric, err := s.commandRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	if err := fabric.UpdateFabric(name, measureUnit, offerStatus, version); err != nil {
		return nil, err
	}

	if err := s.commandRepo.Update(ctx, fabric); err != nil {
		wrappedErr := fmt.Errorf("failed to update fabric in repo: %w", err)
		logger.Error("updating fabric failed", "error", wrappedErr)
		span.RecordError(wrappedErr)
		span.SetStatus(codes.Error, "database write error")
		return nil, wrappedErr
	}

	var envelopesToPublish []*messaging.EventEnvelope
	for _, event := range fabric.Events() {
		if _, ok := event.(domain.FabricUpdated); ok {
			envelope := messaging.NewEventEnvelope(
				"app.fabric.updated",
				fabric.Code,
				"Fabric",
				fabric.Version,
				event,
			)
			envelopesToPublish = append(envelopesToPublish, envelope)
		}
	}

	if len(envelopesToPublish) > 0 {
		if err := s.eventStore.Save(ctx, envelopesToPublish...); err != nil {
			wrappedErr := fmt.Errorf("failed to save update event to event store: %w", err)
			logger.Error("saving update event to event store failed", "error", wrappedErr)
			span.RecordError(wrappedErr)
			span.SetStatus(codes.Error, "event store write error")
			return nil, wrappedErr
		}

		if command.IsFromREST(ctx) {
			for _, envelope := range envelopesToPublish {
				if err := s.publisher.Publish(ctx, s.eventChannel, envelope); err != nil {
					wrappedErr := fmt.Errorf("failed to publish fabric updated event: %w", err)
					logger.Error("publishing fabric updated event failed", "error", wrappedErr, "eventID", envelope.EventID)
					span.RecordError(wrappedErr)
				}
			}
		}
	}

	return fabric, nil
}

func (s *FabricService) DeleteFabric(ctx context.Context, code string, version int) error {
	ctx, span := otel.Tracer("s-works/api").Start(ctx, "fabric.service.delete")
	defer span.End()
	logger := httpx.GetLogger(ctx).With("component", "fabric.service")

	fabric, err := s.commandRepo.GetByCode(ctx, code)
	if err != nil {
		return err
	}

	if err := fabric.Delete(version); err != nil {
		return err
	}

	if err := s.commandRepo.Delete(ctx, fabric); err != nil {
		wrappedErr := fmt.Errorf("failed to delete fabric in repo: %w", err)
		logger.Error("deleting fabric failed", "error", wrappedErr)
		span.RecordError(wrappedErr)
		return wrappedErr
	}

	var envelopesToPublish []*messaging.EventEnvelope
	for _, event := range fabric.Events() {
		if _, ok := event.(domain.FabricDeleted); ok {
			envelope := messaging.NewEventEnvelope(
				"app.fabric.deleted",
				fabric.Code,
				"Fabric",
				fabric.Version,
				event,
			)
			envelopesToPublish = append(envelopesToPublish, envelope)
		}
	}

	if len(envelopesToPublish) > 0 {
		if err := s.eventStore.Save(ctx, envelopesToPublish...); err != nil {
			wrappedErr := fmt.Errorf("failed to save delete event to event store: %w", err)
			logger.Error("saving delete event failed", "error", wrappedErr)
			span.RecordError(wrappedErr)
			return wrappedErr
		}
		// the contextet may be from REST API or from NATS subscription
		if command.IsFromREST(ctx) {
			for _, envelope := range envelopesToPublish {
				if err := s.publisher.Publish(ctx, s.eventChannel, envelope); err != nil {
					wrappedErr := fmt.Errorf("failed to publish fabric deleted event: %w", err)
					logger.Error("publishing fabric deleted event failed", "error", wrappedErr, "eventID", envelope.EventID)
					span.RecordError(wrappedErr)
				}
			}
		}
	}

	return nil
}

func (s *FabricService) GetByCode(ctx context.Context, code string) (*domain.Fabric, error) {
	return s.commandRepo.GetByCode(ctx, code)
}

func (s *FabricService) GetByCodeIncludingDeleted(ctx context.Context, code string) (*domain.Fabric, error) {
	return s.commandRepo.GetByCodeIncludingDeleted(ctx, code)
}
