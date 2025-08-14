package handler

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"github.com/salesworks/s-works/api/internal/fabrics/domain"
	command "github.com/salesworks/s-works/api/internal/platform/context"
	"github.com/salesworks/s-works/api/internal/platform/httpx"
	"github.com/salesworks/s-works/api/internal/platform/validator"
)

type FabricCommandService interface {
	CreateFabric(
		ctx context.Context, code, name, measureUnit, offerStatus string,
	) (*domain.Fabric, error)
	UpdateFabric(
		ctx context.Context, code, name, measureUnit, offerStatus string, version int,
	) (*domain.Fabric, error)
	DeleteFabric(ctx context.Context, code string, version int) error
	GetByCode(ctx context.Context, code string) (*domain.Fabric, error)
}

type FabricCommandHandler struct {
	service FabricCommandService
}

// data contract for API endpoint
type createFabricRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	MeasureUnit string `json:"measure_unit"`
	OfferStatus string `json:"offer_status"`
}

type updateFabricRequest struct {
	Name        string `json:"name"`
	MeasureUnit string `json:"measure_unit"`
	OfferStatus string `json:"offer_status"`
	Version     int    `json:"version"`
}

type deleteFabricRequest struct {
	Version int `json:"version"`
}

func NewFabricCommandHandler(service FabricCommandService) *FabricCommandHandler {
	return &FabricCommandHandler{
		service: service,
	}
}

func (h *FabricCommandHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.createFabric(w, r)
	case http.MethodPut:
		h.updateFabric(w, r)
	case http.MethodDelete: // NEW: Handle DELETE requests.
		h.deleteFabric(w, r)
	default:
		httpx.MethodNotAllowed(w, r)
	}
}

func (h *FabricCommandHandler) createFabric(w http.ResponseWriter, r *http.Request) {
	ctx := command.WithCommandSource(r.Context(), command.CommandSourceREST)

	var req createFabricRequest
	if err := httpx.ReadJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	validateCreateFabricRequest(v, &req)
	if !v.Valid() {
		httpx.ValidationError(w, r, v.Errors)
		return
	}

	_, err := h.service.CreateFabric(
		ctx,
		req.Code,
		req.Name,
		req.MeasureUnit,
		req.OfferStatus,
	)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDuplicateFabricCode):
			httpx.ErrorJSON(w, http.StatusConflict, "a fabric with this code already exists")
		case errors.Is(err, domain.ErrInvalidFabricCodeLength) ||
			errors.Is(err, domain.ErrInvalidFabricCodePattern) ||
			errors.Is(err, domain.ErrInvalidFabricNameLength):
			httpx.ValidationError(w, r, map[string]string{"error": err.Error()})
		default:
			httpx.InternalError(w, r, err)
		}
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *FabricCommandHandler) updateFabric(w http.ResponseWriter, r *http.Request) {
	ctx := command.WithCommandSource(r.Context(), command.CommandSourceREST)

	code := httpx.URLParam(r, "code")

	var req updateFabricRequest
	if err := httpx.ReadJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	validateUpdateFabricRequest(v, &req)
	if !v.Valid() {
		httpx.ValidationError(w, r, v.Errors)
		return
	}

	_, err := h.service.UpdateFabric(
		ctx,
		code,
		req.Name,
		req.MeasureUnit,
		req.OfferStatus,
		req.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrRecordNotFound):
			httpx.NotFound(w, r)
		case errors.Is(err, domain.ErrConcurrencyConflict):
			httpx.ErrorJSON(w, http.StatusConflict, "the resource has been modified by another process, please refresh and try again")
		case errors.Is(err, domain.ErrInvalidFabricNameLength):
			httpx.ValidationError(w, r, map[string]string{"error": err.Error()})
		default:
			httpx.InternalError(w, r, err)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *FabricCommandHandler) deleteFabric(w http.ResponseWriter, r *http.Request) {
	ctx := command.WithCommandSource(r.Context(), command.CommandSourceREST)

	code := httpx.URLParam(r, "code")

	var req deleteFabricRequest
	if err := httpx.ReadJSON(w, r, &req); err != nil {
		httpx.BadRequest(w, r, err)
		return
	}

	v := validator.New()
	v.Check(req.Version > 0, "version", "version must be provided and greater than 0")
	if !v.Valid() {
		httpx.ValidationError(w, r, v.Errors)
		return
	}

	err := h.service.DeleteFabric(ctx, code, req.Version)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrRecordNotFound):
			httpx.NotFound(w, r)
		case errors.Is(err, domain.ErrConcurrencyConflict):
			httpx.ErrorJSON(w, http.StatusConflict, "the resource has been modified by another process, please refresh and try again")
		default:
			httpx.InternalError(w, r, err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func validateCreateFabricRequest(v *validator.Validator, req *createFabricRequest) {
	// --- Fabric Code Validation ---
	v.Check(req.Code != "", "code", "code must be provided")
	v.Check(len(req.Code) >= 2, "code", "code must be between 2 and 30 characters long")
	v.Check(len(req.Code) <= 30, "code", "code must be between 2 and 30 characters long")
	v.Check(validator.Matches(req.Code, regexp.MustCompile("^[A-Z0-9]+$")), "code", "code must only contain uppercase letters and numbers")

	// --- Fabric Name Validation ---
	v.Check(req.Name != "", "name", "name must be provided")
	v.Check(len(req.Name) <= 250, "name", "name must not be more than 250 characters long")
}

func validateUpdateFabricRequest(v *validator.Validator, req *updateFabricRequest) {
	v.Check(req.Version > 0, "version", "version must be provided and greater than 0")
	v.Check(req.Name != "", "name", "name must be provided")
	v.Check(len(req.Name) <= 250, "name", "name must not be more than 250 characters long")
}
