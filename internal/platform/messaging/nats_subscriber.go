package messaging

import (
	"context"
	"log/slog"

	"github.com/nats-io/nats.go"
)

// MessageHandler is the interface that any application-level handler must implement.
// It receives the raw message data from the subscriber.
type MessageHandler interface {
	HandleMessage(ctx context.Context, subject string, payload []byte) error
}

// NatsSubscriber manages a NATS subscription and delegates message processing.
type NatsSubscriber struct {
	conn       *nats.Conn
	handler    MessageHandler
	subject    string
	queueGroup string
	logger     *slog.Logger
}

// NewNatsSubscriber creates and initializes a new NatsSubscriber.
func NewNatsSubscriber(
	conn *nats.Conn,
	handler MessageHandler,
	subject string,
	queueGroup string,
	logger *slog.Logger,
) *NatsSubscriber {
	return &NatsSubscriber{
		conn:       conn,
		handler:    handler,
		subject:    subject,
		queueGroup: queueGroup,
		logger:     logger.With("component", "natsSubscriber"),
	}
}

// StartListening creates a subscription and processes messages in the background.
func (s *NatsSubscriber) StartListening() {
	s.conn.QueueSubscribe(s.subject, s.queueGroup, func(msg *nats.Msg) {
		s.logger.Debug("Received message", "subject", msg.Subject)

		// Delegate all logic to the injected handler.
		if err := s.handler.HandleMessage(context.Background(), msg.Subject, msg.Data); err != nil {
			s.logger.Error("Failed to handle message", "error", err)
			return
		}

		s.logger.Info("Successfully processed message", "subject", msg.Subject)
	})
}
