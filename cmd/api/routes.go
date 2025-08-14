package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	fabricHandler "github.com/salesworks/s-works/api/internal/fabrics/handler"
	"github.com/salesworks/s-works/api/internal/platform/httpx"
)

func (api *api) routes(metricsHandler http.Handler) http.Handler {
	router := chi.NewRouter()

	// Apply panic recovery first to catch anything below it
	router.Use(httpx.RecoverPanic(api.logger))

	// Inject request_id and per-request logger
	router.Use(httpx.RequestLoggerMiddleware(api.logger))

	// Inject system context
	router.Use(httpx.SystemContextMiddleware(api.config.env, version))

	// --- Public / Ungrouped Routes ---
	router.Method(http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	router.Method(http.MethodGet, "/metrics", metricsHandler)

	// --- V1 API Route Group (clerk middleware) ---
	router.Route("/v1", func(r chi.Router) {
		// --- Write Endpoint ---
		fh := fabricHandler.NewFabricCommandHandler(api.services.FabricCommandService)
		r.Method(http.MethodPost, "/fabrics", fh)
		r.Method(http.MethodPut, "/fabrics/{code}", fh)
		r.Method(http.MethodDelete, "/fabrics/{code}", fh)

		// --- Read Endpoint ---
		fqh := fabricHandler.NewFabricQueryHandler(api.repositories.FabricQueryRepository)
		r.Method(http.MethodGet, "/fabrics/{code}", fqh)
	})

	return router
}
