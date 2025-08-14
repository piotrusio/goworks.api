package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/salesworks/s-works/api/internal/fabrics/domain"
	"github.com/stretchr/testify/assert"
)

type mockFabricQueryRepository struct {
	fabricToReturn *domain.Fabric
	errorToReturn  error
}

func (m *mockFabricQueryRepository) GetByCode(ctx context.Context, code string) (*domain.Fabric, error) {
	return m.fabricToReturn, m.errorToReturn
}

func TestFabricQueryHandler_GetByCode_HappyPath(t *testing.T) {
	// --- Arrange ---
	expectedFabric := &domain.Fabric{
		Code:        "EXISTING",
		Name:        "An Existing Fabric",
		MeasureUnit: "m",
		OfferStatus: "available",
	}

	mockRepo := &mockFabricQueryRepository{
		fabricToReturn: expectedFabric,
		errorToReturn:  nil,
	}

	handler := NewFabricQueryHandler(mockRepo)
	req, err := http.NewRequest(http.MethodGet, "/v1/fabrics/EXISTING", nil)
	assert.NoError(t, err)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", "EXISTING")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	responseRecorder := httptest.NewRecorder()

	// --- Act ---
	handler.ServeHTTP(responseRecorder, req)

	// --- Assert ---
	assert.Equal(t, http.StatusOK, responseRecorder.Code)

	var responseEnvelope struct {
		Fabric domain.Fabric `json:"fabric"`
	}
	err = json.Unmarshal(responseRecorder.Body.Bytes(), &responseEnvelope)
	assert.NoError(t, err)

	actualFabric := responseEnvelope.Fabric
	assert.Equal(t, expectedFabric.Code, actualFabric.Code)
	assert.Equal(t, expectedFabric.Name, actualFabric.Name)
}
