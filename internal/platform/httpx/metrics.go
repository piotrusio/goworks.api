package httpx

import (
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter                  = otel.Meter("s-works/api")
	httpRequestDuration    metric.Float64Histogram
	httpRequestCounter     metric.Int64Counter
	FabricGetByCodeCounter metric.Int64Counter
)

func init() {
	httpRequestDuration, _ = meter.Float64Histogram("http.server.duration")
	httpRequestCounter, _ = meter.Int64Counter("http.server.requests")
	FabricGetByCodeCounter, _ = meter.Int64Counter("fabric.get_by_code.total")
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// wrap ResponseWriter to capture status
		rr := &responseRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rr, r)

		duration := time.Since(start).Seconds()

		labels := []attribute.KeyValue{
			attribute.String("method", r.Method),
			attribute.String("path", r.URL.Path),
			attribute.String("status", strconv.Itoa(rr.status)),
		}

		httpRequestCounter.Add(r.Context(), 1, metric.WithAttributes(labels...))
		httpRequestDuration.Record(r.Context(), duration, metric.WithAttributes(labels...))
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.status = code
	rr.ResponseWriter.WriteHeader(code)
}
