package httpx

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type ctxKey string
type ctxKeyLogger struct{}

const (
	ctxKeyEnv     ctxKey = "env"
	ctxKeyVersion ctxKey = "version"
)

func RecoverPanic(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					err := fmt.Errorf("%s", rec)

					// Record the error on the active span
					span := trace.SpanFromContext(r.Context())
					span.RecordError(err)
					span.SetStatus(codes.Error, "panic recovered")

					// Respond and log as before
					w.Header().Set("Connection", "close")
					InternalError(w, r, err)
					logger.Error("panic recovered", "panic", rec)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// injects a per-request logger into the context, includes request_id, method, and path in the logger fields
func RequestLoggerMiddleware(baseLogger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := uuid.NewString()

			// a global propagator will automatically be used to check for incoming headers
			// (like x-cloud-trace-context) and link this new span to the parent trace if one exists.
			ctx, span := otel.Tracer("s-works/api").Start(r.Context(), r.URL.Path)
			defer span.End()

			spanID := span.SpanContext().SpanID().String()
			traceID := span.SpanContext().TraceID().String()

			logger := baseLogger.With(
				"request_id", requestID,
				"trace_id", traceID,
				"span_id", spanID,
				"method", r.Method,
				"path", r.URL.Path,
			)

			ctx = context.WithValue(ctx, ctxKeyLogger{}, logger)
			r = r.WithContext(ctx)

			logger.Info("request started")

			rw := &statusResponseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)

			logger.Info("request finished", "status", rw.status)
		})
	}
}

// extracts the request-scoped logger from the context, falls back to slog.Default() if no logger is present
func GetLogger(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(ctxKeyLogger{}).(*slog.Logger)
	if !ok {
		return slog.Default()
	}
	return logger
}

func SystemContextMiddleware(env, version string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKeyEnv, env)
			ctx = context.WithValue(ctx, ctxKeyVersion, version)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func SystemEnv(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyEnv).(string); ok {
		return v
	}
	return "unknown"
}

func SystemVersion(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyVersion).(string); ok {
		return v
	}
	return "unknown"
}

func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger{}, logger)
}
