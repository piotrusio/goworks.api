package command

import "context"

// CommandSource represents where a command originated from
type CommandSource string

const (
	CommandSourceREST  CommandSource = "rest"  // From REST API
	CommandSourceEvent CommandSource = "event" // From NATS event
)

// Internal context key type to avoid collisions
type contextKey string

const commandSourceKey contextKey = "command_source"

// WithCommandSource adds the command source to context
func WithCommandSource(ctx context.Context, source CommandSource) context.Context {
	return context.WithValue(ctx, commandSourceKey, source)
}

// GetCommandSource retrieves the command source from context
func GetCommandSource(ctx context.Context) CommandSource {
	if source, ok := ctx.Value(commandSourceKey).(CommandSource); ok {
		return source
	}
	return CommandSourceREST // Safe default
}

// IsFromREST checks if the command came from REST API
func IsFromREST(ctx context.Context) bool {
	return GetCommandSource(ctx) == CommandSourceREST
}

// IsFromEvent checks if the command came from an event handler
func IsFromEvent(ctx context.Context) bool {
	return GetCommandSource(ctx) == CommandSourceEvent
}
