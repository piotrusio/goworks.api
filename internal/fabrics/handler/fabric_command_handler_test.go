package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/salesworks/s-works/api/internal/fabrics/domain"
	"github.com/stretchr/testify/assert"
)

type mockFabricCommandService struct {
	CreateFabricCalled bool
	UpdateFabricCalled bool
	DeleteFabricCalled bool
	GetByCodeCalled    bool
	errToReturn        error
}

func (m *mockFabricCommandService) CreateFabric(
	ctx context.Context, code, name, measureUnit, offerStatus string,
) (*domain.Fabric, error) {
	m.CreateFabricCalled = true
	return &domain.Fabric{Code: code}, m.errToReturn
}

func (m *mockFabricCommandService) UpdateFabric(
	ctx context.Context, code, name, measureUnit, offerStatus string, version int,
) (*domain.Fabric, error) {
	m.UpdateFabricCalled = true
	if m.errToReturn != nil {
		return nil, m.errToReturn
	}
	return &domain.Fabric{Code: code, Name: name}, nil
}

func (m *mockFabricCommandService) DeleteFabric(ctx context.Context, code string, version int) error {
	m.DeleteFabricCalled = true
	return m.errToReturn
}

func (m *mockFabricCommandService) GetByCode(ctx context.Context, code string) (*domain.Fabric, error) {
	m.GetByCodeCalled = true
	if m.errToReturn != nil {
		return nil, m.errToReturn
	}
	return &domain.Fabric{Code: code}, nil
}

func TestFabricCommandHandler_CreateFabric_HappyPath(t *testing.T) {
	// --- Arrange ---
	mockSvc := &mockFabricCommandService{}
	handler := NewFabricCommandHandler(mockSvc)

	requestBody := `{"code": "TEST01", "name": "Test Name", "measure_unit": "mb", "offer_status": "new"}`
	request, err := http.NewRequest(http.MethodPost, "/v1/fabrics", strings.NewReader(requestBody))
	assert.NoError(t, err)

	// --- Act ---
	responseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(responseRecorder, request)

	// --- Assert ---
	assert.True(t, mockSvc.CreateFabricCalled, "expected CreateFabric to be called on the service")
	assert.Equal(t, http.StatusAccepted, responseRecorder.Code, "expected HTTP status 202 Accepted")
}

func TestFabricCommandHandler_CreateFabric_ValidationErrors(t *testing.T) {
	testCases := []struct {
		name                 string
		body                 string
		expectedStatusCode   int
		expectedErrorSnippet string
	}{
		{
			name:                 "Empty Body Request",
			body:                 "",
			expectedStatusCode:   http.StatusBadRequest,
			expectedErrorSnippet: "body must not be empty",
		},
		{
			name:                 "MalformedJSON",
			body:                 `{"code": "TEST01", "name": "Test Name",}`, // invalid trailing comma
			expectedStatusCode:   http.StatusBadRequest,
			expectedErrorSnippet: "body contains badly-formed JSON",
		},
		{
			name:                 "Incorrect data type",
			body:                 `{"code": 123, "name": "Test Name"}`, // code is a number
			expectedStatusCode:   http.StatusBadRequest,
			expectedErrorSnippet: "body contains incorrect JSON type",
		},
		{
			name:                 "Unknown Field in Body",
			body:                 `{"code": "TEST01", "color": "Blue"}`, // "colour" is unknown
			expectedStatusCode:   http.StatusBadRequest,
			expectedErrorSnippet: "body contains unknown key",
		},
		{
			name:                 "Code is empty",
			body:                 `{"code": "", "name": "Test Fabric"}`,
			expectedStatusCode:   http.StatusUnprocessableEntity,
			expectedErrorSnippet: "code must be provided",
		},
		{
			name:                 "Code is too long",
			body:                 `{"code": "ABCDEFGHIJKLMNOPQRSTUVWXYZ12345", "name": "Test Fabric"}`,
			expectedStatusCode:   http.StatusUnprocessableEntity, // 422
			expectedErrorSnippet: "code must be between 2 and 30 characters long",
		},
		{
			name:                 "Name is empty",
			body:                 `{"code": "TEST01", "name": ""}`,
			expectedStatusCode:   http.StatusUnprocessableEntity,
			expectedErrorSnippet: "name must be provided",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// --- Arrange ---
			mockSvc := &mockFabricCommandService{}
			handler := NewFabricCommandHandler(mockSvc)

			request, err := http.NewRequest(http.MethodPost, "/v1/fabrics", strings.NewReader(tc.body))
			assert.NoError(t, err)

			responseRecorder := httptest.NewRecorder()

			// --- Act ---
			handler.ServeHTTP(responseRecorder, request)

			// --- Assert ---
			assert.Equal(t, tc.expectedStatusCode, responseRecorder.Code)
			assert.Contains(t, responseRecorder.Body.String(), tc.expectedErrorSnippet)
			assert.False(t, mockSvc.CreateFabricCalled, "service should not be called with invalid input")
		})
	}
}

func TestFabricCommandHandler_CreateFabric_DuplicateCode(t *testing.T) {
	// --- Arrange ---
	mockSvc := &mockFabricCommandService{errToReturn: domain.ErrDuplicateFabricCode}
	handler := NewFabricCommandHandler(mockSvc)

	requestBody := `{"code": "DUPLICATE", "name": "Duplicate Fabric", "measure_unit": "m", "offer_status": "new"}`
	request, err := http.NewRequest(http.MethodPost, "/v1/fabrics", strings.NewReader(requestBody))
	assert.NoError(t, err)

	responseRecorder := httptest.NewRecorder()

	// --- Act ---
	handler.ServeHTTP(responseRecorder, request)

	// --- Assert ---
	assert.Equal(t, http.StatusConflict, responseRecorder.Code, "expected HTTP status 409 Conflict")
}

func TestFabricCommandHandler_UpdateFabric_HappyPath(t *testing.T) {
	// --- Arrange ---
	mockSvc := &mockFabricCommandService{}
	handler := NewFabricCommandHandler(mockSvc)

	requestBody := `{"name": "Updated Name", "measure_unit": "cm", "offer_status": "new", "version": 1}`
	request, err := http.NewRequest(http.MethodPut, "/v1/fabrics/TEST01", strings.NewReader(requestBody))
	assert.NoError(t, err)

	// To handle the URL param, we must add it to the request context, simulating a router.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", "TEST01")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

	responseRecorder := httptest.NewRecorder()

	// --- Act ---
	handler.ServeHTTP(responseRecorder, request)

	// --- Assert ---
	assert.True(t, mockSvc.UpdateFabricCalled, "expected UpdateFabric to be called on the service")
	assert.Equal(t, http.StatusOK, responseRecorder.Code, "expected HTTP status 200 OK")
}

func TestFabricCommandHandler_UpdateFabric_NotFound(t *testing.T) {
	// --- Arrange ---
	mockSvc := &mockFabricCommandService{errToReturn: domain.ErrRecordNotFound}
	handler := NewFabricCommandHandler(mockSvc)

	requestBody := `{"name": "Updated Name", "version": 1}`
	request, err := http.NewRequest(http.MethodPut, "/v1/fabrics/NONEXISTENT", strings.NewReader(requestBody))
	assert.NoError(t, err)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", "NONEXISTENT")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

	responseRecorder := httptest.NewRecorder()

	// --- Act ---
	handler.ServeHTTP(responseRecorder, request)

	// --- Assert ---
	assert.Equal(t, http.StatusNotFound, responseRecorder.Code, "expected HTTP status 404 Not Found")
}

func TestFabricCommandHandler_UpdateFabric_ConcurrencyConflict(t *testing.T) {
	// --- Arrange ---
	mockSvc := &mockFabricCommandService{errToReturn: domain.ErrConcurrencyConflict}
	handler := NewFabricCommandHandler(mockSvc)

	requestBody := `{"name": "Updated Name", "version": 1}` // Client sends version 1
	request, err := http.NewRequest(http.MethodPut, "/v1/fabrics/TEST01", strings.NewReader(requestBody))
	assert.NoError(t, err)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", "TEST01")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

	responseRecorder := httptest.NewRecorder()

	// --- Act ---
	handler.ServeHTTP(responseRecorder, request)

	// --- Assert ---
	assert.Equal(t, http.StatusConflict, responseRecorder.Code, "expected HTTP status 409 Conflict")
	assert.Contains(t, responseRecorder.Body.String(), "resource has been modified")
}

func TestFabricCommandHandler_UpdateFabric_ValidationErrors(t *testing.T) {
	testCases := []struct {
		name                 string
		body                 string
		expectedErrorSnippet string
	}{
		{
			name:                 "Missing Version",
			body:                 `{"name": "Test Fabric"}`,
			expectedErrorSnippet: "version must be provided",
		},
		{
			name:                 "Missing Name",
			body:                 `{"version": 1}`,
			expectedErrorSnippet: "name must be provided",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// --- Arrange ---
			mockSvc := &mockFabricCommandService{}
			handler := NewFabricCommandHandler(mockSvc)

			request, err := http.NewRequest(http.MethodPut, "/v1/fabrics/TEST01", strings.NewReader(tc.body))
			assert.NoError(t, err)

			responseRecorder := httptest.NewRecorder()

			// --- Act ---
			handler.ServeHTTP(responseRecorder, request)

			// --- Assert ---
			assert.Equal(t, http.StatusUnprocessableEntity, responseRecorder.Code)
			assert.Contains(t, responseRecorder.Body.String(), tc.expectedErrorSnippet)
			assert.False(t, mockSvc.UpdateFabricCalled, "service should not be called with invalid input")
		})
	}
}

func TestFabricCommandHandler_DeleteFabric_HappyPath(t *testing.T) {
	// --- Arrange ---
	mockSvc := &mockFabricCommandService{}
	handler := NewFabricCommandHandler(mockSvc)

	requestBody := `{"version": 1}`
	request, err := http.NewRequest(http.MethodDelete, "/v1/fabrics/DELETEME", strings.NewReader(requestBody))
	assert.NoError(t, err)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", "DELETEME")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

	responseRecorder := httptest.NewRecorder()

	// --- Act ---
	handler.ServeHTTP(responseRecorder, request)

	// --- Assert ---
	assert.True(t, mockSvc.DeleteFabricCalled, "expected DeleteFabric to be called on the service")
	assert.Equal(t, http.StatusNoContent, responseRecorder.Code, "expected HTTP status 204 No Content")
}

func TestFabricCommandHandler_DeleteFabric_NotFound(t *testing.T) {
	// --- Arrange ---
	mockSvc := &mockFabricCommandService{errToReturn: domain.ErrRecordNotFound}
	handler := NewFabricCommandHandler(mockSvc)

	requestBody := `{"version": 1}`
	request, err := http.NewRequest(http.MethodDelete, "/v1/fabrics/NONEXISTENT", strings.NewReader(requestBody))
	assert.NoError(t, err)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", "NONEXISTENT")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

	responseRecorder := httptest.NewRecorder()

	// --- Act ---
	handler.ServeHTTP(responseRecorder, request)

	// --- Assert ---
	assert.True(t, mockSvc.DeleteFabricCalled, "expected DeleteFabric to be called")
	assert.Equal(t, http.StatusNotFound, responseRecorder.Code, "expected HTTP status 404 Not Found")
}

func TestFabricCommandHandler_DeleteFabric_ConcurrencyConflict(t *testing.T) {
	// --- Arrange ---
	mockSvc := &mockFabricCommandService{errToReturn: domain.ErrConcurrencyConflict}
	handler := NewFabricCommandHandler(mockSvc)

	requestBody := `{"version": 1}` // Stale version
	request, err := http.NewRequest(http.MethodDelete, "/v1/fabrics/CONFLICT", strings.NewReader(requestBody))
	assert.NoError(t, err)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", "CONFLICT")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, rctx))

	responseRecorder := httptest.NewRecorder()

	// --- Act ---
	handler.ServeHTTP(responseRecorder, request)

	// --- Assert ---
	assert.True(t, mockSvc.DeleteFabricCalled, "expected DeleteFabric to be called")
	assert.Equal(t, http.StatusConflict, responseRecorder.Code, "expected HTTP status 409 Conflict")
}
