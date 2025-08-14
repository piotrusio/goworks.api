package bootstrap

import (
	"log/slog"

	"github.com/nats-io/nats.go"
	fabricApp "github.com/salesworks/s-works/api/internal/fabrics/application"
	"github.com/salesworks/s-works/api/internal/fabrics/handler"
	"github.com/salesworks/s-works/api/internal/platform/eventstore"
	"github.com/salesworks/s-works/api/internal/platform/messaging"
)

type Services struct {
	FabricCommandService handler.FabricCommandService
}

func NewServices(
	repositories Repositories, natsConn *nats.Conn, logger *slog.Logger,
) Services {
	appEventPublisher := messaging.NewNatsPublisher(natsConn, logger)
	eventStore := eventstore.NewPostgresStore(repositories.postgres.Pool)
	fabricCommandService := fabricApp.NewFabricCommandService(
		repositories.FabricCommandRepository,
		appEventPublisher,
		eventStore,
	)

	return Services{
		FabricCommandService: fabricCommandService,
	}
}
