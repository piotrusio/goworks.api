package main

import (
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/salesworks/s-works/api/internal/bootstrap"
	"github.com/salesworks/s-works/api/internal/fabrics/handler"
	"github.com/salesworks/s-works/api/internal/platform/messaging"
)

// Subscribers holds the dependencies required for message processing.
type Subscribers struct {
	natsConn *nats.Conn
	services bootstrap.Services
	logger   *slog.Logger
}

// NewSubscribers creates a new instance of our subscriber manager.
func NewSubscribers(natsConn *nats.Conn, services bootstrap.Services, logger *slog.Logger) *Subscribers {
	return &Subscribers{
		natsConn: natsConn,
		services: services,
		logger:   logger,
	}
}

// Start begins listening for messages on all configured subjects.
// It should be run as a goroutine.
func (s *Subscribers) Start() {
	// Create the message router
	router := messaging.NewMessageRouter(s.logger)

	// Register handlers with the router
	fabricEventHandler := handler.NewFabricEventHandler(s.services.FabricCommandService, s.logger)
	router.RegisterHandler("erp.fabric", fabricEventHandler)

	// Create a single subscriber that uses the router
	natsSubscriber := messaging.NewNatsSubscriber(
		s.natsConn,
		router,
		"erp.*",             // Wildcard to catch all ERP events
		"erp-service-group", // TODO: Get from config
		s.logger,
	)

	s.logger.Info("starting NATS subscribers with router")
	natsSubscriber.StartListening()
}
