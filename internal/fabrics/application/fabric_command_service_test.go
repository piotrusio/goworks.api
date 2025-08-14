package application

import (
	"context"
	"testing"

	"github.com/salesworks/s-works/api/internal/fabrics/domain"
	"github.com/salesworks/s-works/api/internal/platform/messaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFabricCommandRepository struct {
	SavedCalled  bool
	UpdateCalled bool
	DeleteCalled bool
	fabric       *domain.Fabric
	errToReturn  error
}

func (m *mockFabricCommandRepository) Save(ctx context.Context, fabric *domain.Fabric) (*domain.Fabric, error) {
	if m.errToReturn != nil {
		return nil, m.errToReturn
	}
	m.SavedCalled = true
	m.fabric = fabric
	return fabric, nil
}

func (m *mockFabricCommandRepository) GetByCode(ctx context.Context, code string) (*domain.Fabric, error) {
	if m.errToReturn != nil {
		return nil, m.errToReturn
	}
	if m.fabric != nil && m.fabric.Code == code && m.fabric.Status == domain.StatusActive {
		fabricCopy := *m.fabric
		return &fabricCopy, nil
	}
	return nil, domain.ErrRecordNotFound
}

func (m *mockFabricCommandRepository) GetByCodeIncludingDeleted(ctx context.Context, code string) (*domain.Fabric, error) {
	if m.errToReturn != nil {
		return nil, m.errToReturn
	}
	if m.fabric != nil && m.fabric.Code == code {
		fabricCopy := *m.fabric
		return &fabricCopy, nil
	}
	return nil, domain.ErrRecordNotFound
}

func (m *mockFabricCommandRepository) Update(ctx context.Context, fabric *domain.Fabric) error {
	if m.errToReturn != nil {
		return m.errToReturn
	}
	m.UpdateCalled = true
	m.fabric = fabric
	return nil
}

func (m *mockFabricCommandRepository) Delete(ctx context.Context, fabric *domain.Fabric) error {
	if m.errToReturn != nil {
		return m.errToReturn
	}
	m.DeleteCalled = true
	m.fabric.Status = domain.StatusDeleted
	return nil
}

type mockEventPublisher struct {
	PublishedCalled   bool
	PublishedEnvelope *messaging.EventEnvelope
}

func (m *mockEventPublisher) Publish(ctx context.Context, subject string, envelope *messaging.EventEnvelope) error {
	m.PublishedCalled = true
	m.PublishedEnvelope = envelope
	return nil
}

func (m *mockEventPublisher) Close() error {
	// No-op for mock
	return nil
}

type mockEventStore struct {
	SavedCalled bool
}

func (m *mockEventStore) Save(ctx context.Context, envelopes ...*messaging.EventEnvelope) error {
	m.SavedCalled = true
	return nil
}

func TestFabricService_CreateFabric_HappyPath(t *testing.T) {
	// --- Arrange ---
	commandRepo := &mockFabricCommandRepository{}
	publisher := &mockEventPublisher{}
	eventStore := &mockEventStore{}
	service := NewFabricCommandService(commandRepo, publisher, eventStore)

	ctx := context.Background()
	code := "TESTCODE"
	name := "Test Fabric"
	measureUnit := "mb"
	offerStatus := "available"

	// --- Act ---
	createdFabric, err := service.CreateFabric(ctx, code, name, measureUnit, offerStatus)

	// --- Assert ---
	assert.NoError(t, err)
	assert.NotNil(t, createdFabric)
	assert.True(t, commandRepo.SavedCalled, "expected Save() to be called on the write repository")
	assert.True(t, publisher.PublishedCalled, "expected Publish() to be called on the event publisher")
	assert.True(t, eventStore.SavedCalled, "expected Save() to be called on the event store")

	publishedEnvelope := publisher.PublishedEnvelope
	require.NotNil(t, publishedEnvelope, "published envelope should not be nil")
	assert.Equal(t, "app.fabric.created", publishedEnvelope.EventType)
	assert.Equal(t, "Fabric", publishedEnvelope.AggregateType)
	assert.Equal(t, code, publishedEnvelope.AggregateID)
	assert.Equal(t, 1, publishedEnvelope.AggregateVersion)

	payload, ok := publishedEnvelope.Payload.(domain.FabricCreated)
	require.True(t, ok, "payload should be of type domain.FabricCreated")
	assert.Equal(t, code, payload.Code)
	assert.Equal(t, name, payload.Name)
}

func TestFabricService_UpdateFabric_HappyPath(t *testing.T) {
	// --- Arrange ---
	commandRepo := &mockFabricCommandRepository{}
	publisher := &mockEventPublisher{}
	eventStore := &mockEventStore{}
	service := NewFabricCommandService(commandRepo, publisher, eventStore)

	ctx := context.Background()
	code := "TESTCODE"
	initialName := "Initial Fabric"

	existingFabric, err := domain.NewFabric(code, initialName, "m", "available")
	require.NoError(t, err)
	commandRepo.fabric = existingFabric
	initialVersion := existingFabric.Version

	updatedName := "Updated Fabric"
	updatedMeasureUnit := "cm"
	updatedOfferStatus := "out_of_stock"

	// --- Act ---
	updatedFabric, err := service.UpdateFabric(ctx, code, updatedName, updatedMeasureUnit, updatedOfferStatus, initialVersion)

	// --- Assert ---
	require.NoError(t, err)
	assert.NotNil(t, updatedFabric)
	assert.True(t, commandRepo.UpdateCalled, "expected Update() to be called on the repository")
	assert.True(t, eventStore.SavedCalled, "expected Save() to be called on the event store")
	assert.True(t, publisher.PublishedCalled, "expected Publish() to be called on the event publisher")

	publishedEnvelope := publisher.PublishedEnvelope
	require.NotNil(t, publishedEnvelope, "published envelope should not be nil")
	assert.Equal(t, "app.fabric.updated", publishedEnvelope.EventType)
	assert.Equal(t, code, publishedEnvelope.AggregateID)
	assert.Equal(t, initialVersion+1, publishedEnvelope.AggregateVersion)
}

func TestFabricService_UpdateFabric_ConcurrencyError(t *testing.T) {
	// --- Arrange ---
	commandRepo := &mockFabricCommandRepository{}
	publisher := &mockEventPublisher{}
	eventStore := &mockEventStore{}
	service := NewFabricCommandService(commandRepo, publisher, eventStore)

	ctx := context.Background()
	code := "TESTCODE"
	existingFabric, err := domain.NewFabric(code, "Initial Name", "m", "available")
	require.NoError(t, err)
	commandRepo.fabric = existingFabric

	staleVersion := existingFabric.Version - 1

	// --- Act ---
	_, err = service.UpdateFabric(ctx, code, "New Name", "cm", "new", staleVersion)

	// --- Assert ---
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConcurrencyConflict)
	assert.False(t, commandRepo.UpdateCalled, "Update() should not be called on the repo if domain validation fails")
	assert.False(t, eventStore.SavedCalled, "Save() should not be called on the event store if domain validation fails")
	assert.False(t, publisher.PublishedCalled, "Publish() should not be called if domain validation fails")
}

func TestFabricService_UpdateFabric_NotFound(t *testing.T) {
	// --- Arrange ---
	commandRepo := &mockFabricCommandRepository{errToReturn: domain.ErrRecordNotFound}
	publisher := &mockEventPublisher{}
	eventStore := &mockEventStore{}
	service := NewFabricCommandService(commandRepo, publisher, eventStore)

	ctx := context.Background()

	// --- Act ---
	_, err := service.UpdateFabric(ctx, "NONEXISTENT", "New Name", "cm", "new", 1)

	// --- Assert ---
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrRecordNotFound)
}

func TestFabricService_GetByCode(t *testing.T) {
	// --- Arrange ---
	commandRepo := &mockFabricCommandRepository{}
	publisher := &mockEventPublisher{}
	eventStore := &mockEventStore{}
	service := NewFabricCommandService(commandRepo, publisher, eventStore)

	ctx := context.Background()
	code := "GETBYCODE"
	expectedFabric, _ := domain.NewFabric(code, "Test Fabric", "m", "available")

	commandRepo.fabric = expectedFabric

	// --- Act ---
	retrievedFabric, err := service.GetByCode(ctx, code)

	// --- Assert ---
	require.NoError(t, err)
	require.NotNil(t, retrievedFabric)
	assert.Equal(t, expectedFabric.Code, retrievedFabric.Code)
	assert.Equal(t, expectedFabric.Name, retrievedFabric.Name)
}

func TestFabricService_DeleteFabric_HappyPath(t *testing.T) {
	// --- Arrange ---
	commandRepo := &mockFabricCommandRepository{}
	publisher := &mockEventPublisher{}
	eventStore := &mockEventStore{}
	service := NewFabricCommandService(commandRepo, publisher, eventStore)

	ctx := context.Background()
	code := "DELETEME"
	existingFabric, err := domain.NewFabric(code, "To Be Deleted", "m", "available")
	require.NoError(t, err)
	commandRepo.fabric = existingFabric

	// --- Act ---
	err = service.DeleteFabric(ctx, code, 1)

	// --- Assert ---
	require.NoError(t, err)
	assert.True(t, commandRepo.DeleteCalled, "expected Delete() to be called on the repository")
	assert.True(t, eventStore.SavedCalled, "expected Save() to be called on the event store")
	assert.True(t, publisher.PublishedCalled, "expected Publish() to be called on the event publisher")

	publishedEnvelope := publisher.PublishedEnvelope
	require.NotNil(t, publishedEnvelope)
	assert.Equal(t, "app.fabric.deleted", publishedEnvelope.EventType)
	assert.Equal(t, code, publishedEnvelope.AggregateID)
	assert.Equal(t, 2, publishedEnvelope.AggregateVersion)

	_, ok := publishedEnvelope.Payload.(domain.FabricDeleted)
	require.True(t, ok, "payload should be of type domain.FabricDeleted")
}
