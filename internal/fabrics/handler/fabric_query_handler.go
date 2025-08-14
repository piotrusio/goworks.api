package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/salesworks/s-works/api/internal/fabrics/domain"
	"github.com/salesworks/s-works/api/internal/platform/httpx"
)

type FabricQueryRepository interface {
	GetByCode(ctx context.Context, code string) (*domain.Fabric, error)
}

type FabricQueryHandler struct {
	repo FabricQueryRepository
}

func NewFabricQueryHandler(repo FabricQueryRepository) *FabricQueryHandler {
	return &FabricQueryHandler{
		repo: repo,
	}
}

func (h *FabricQueryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := httpx.URLParam(r, "code")

	fabric, err := h.repo.GetByCode(r.Context(), code)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrRecordNotFound):
			httpx.NotFound(w, r)
		default:
			httpx.InternalError(w, r, err)
		}
		return
	}

	err = httpx.WriteJSON(w, http.StatusOK, httpx.Envelope{"fabric": fabric}, nil)
	if err != nil {
		httpx.InternalError(w, r, err)
	}
}
