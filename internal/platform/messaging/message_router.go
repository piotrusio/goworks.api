package messaging

import (
	"context"
	"log/slog"
	"strings"
)

// MessageRouter routes incoming messages to appropriate handlers based on subject patterns.
type MessageRouter struct {
	handlers map[string]MessageHandler
	logger   *slog.Logger
}

// NewMessageRouter creates a new message router.
func NewMessageRouter(logger *slog.Logger) *MessageRouter {
	return &MessageRouter{
		handlers: make(map[string]MessageHandler),
		logger:   logger.With("component", "messageRouter"),
	}
}

// RegisterHandler registers a handler for a specific subject pattern.
func (r *MessageRouter) RegisterHandler(subjectPattern string, handler MessageHandler) {
	r.handlers[subjectPattern] = handler
	r.logger.Info("Registered message handler", "pattern", subjectPattern)
}

// HandleMessage implements the MessageHandler interface and routes messages to appropriate handlers.
func (r *MessageRouter) HandleMessage(ctx context.Context, subject string, payload []byte) error {
	handler, found := r.findHandler(subject)
	if !found {
		r.logger.Warn("No handler found for subject", "subject", subject)
		return nil
	}

	r.logger.Debug("Routing message", "subject", subject, "handler", "found")
	return handler.HandleMessage(ctx, subject, payload)
}

// findHandler finds the appropriate handler for a given subject.
func (r *MessageRouter) findHandler(subject string) (MessageHandler, bool) {
	// First try exact match
	if handler, exists := r.handlers[subject]; exists {
		return handler, true
	}

	// Then try pattern matching (simple prefix matching for now)
	for pattern, handler := range r.handlers {
		if r.matchesPattern(subject, pattern) {
			return handler, true
		}
	}

	return nil, false
}

// matchesPattern checks if a subject matches a pattern.
// Currently supports simple wildcard patterns like "erp.fabrics" matching "erp.fabrics".
// Can be extended to support more complex patterns in the future.
func (r *MessageRouter) matchesPattern(subject, pattern string) bool {
	// For now, exact match only
	// Future: implement wildcard matching like "erp.*" or "erp.*.created"
	return strings.EqualFold(subject, pattern)
}
