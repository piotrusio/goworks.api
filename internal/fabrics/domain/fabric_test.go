package domain

import (
	"math/rand"
	"testing"
	"time"

	"github.com/go-faker/faker/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFabric_UpdateFabric_HappyPath(t *testing.T) {
	// --- Arrange ---
	fabric, err := NewFabric("TESTCODE", "Original Name", "m", "available")
	require.NoError(t, err, "Test setup should not fail")
	initialVersion := fabric.Version

	updatedName := "Updated Name"
	updatedMeasureUnit := "cm"
	updatedOfferStatus := "unavailable"

	// --- Act ---
	err = fabric.UpdateFabric(updatedName, updatedMeasureUnit, updatedOfferStatus, initialVersion)

	// --- Assert ---
	assert.NoError(t, err)
	assert.Equal(t, updatedName, fabric.Name)
	assert.Equal(t, updatedMeasureUnit, fabric.MeasureUnit)
	assert.Equal(t, updatedOfferStatus, fabric.OfferStatus)
	assert.Equal(t, initialVersion+1, fabric.Version, "Version should be incremented by 1")

	// Check for the FabricUpdated event
	require.Len(t, fabric.events, 2, "There should be two events: Created and Updated")
	updateEvent, ok := fabric.events[1].(FabricUpdated)
	require.True(t, ok, "The second event must be a FabricUpdated event")

	assert.Equal(t, fabric.Code, updateEvent.Code)
	assert.Equal(t, updatedName, updateEvent.Name)
	assert.Equal(t, fabric.Version, updateEvent.Version)
}

func TestFabric_UpdateFabric_ConcurrencyConflict(t *testing.T) {
	// --- Arrange ---
	fabric, err := NewFabric("TESTCODE", "Original Name", "m", "available")
	require.NoError(t, err, "Test setup should not fail")

	staleVersion := fabric.Version - 1 // Simulate a stale version number
	correctVersion := fabric.Version

	// --- Act ---
	// Attempt to update with a stale version
	err = fabric.UpdateFabric("New Name", "cm", "new_status", staleVersion)

	// --- Assert ---
	assert.Error(t, err, "An error should be returned for a version mismatch")
	assert.ErrorIs(t, err, ErrConcurrencyConflict, "The error should be a concurrency conflict error")
	assert.Equal(t, correctVersion, fabric.Version, "Version should not change on a failed update")
	assert.Len(t, fabric.events, 1, "No new event should be added on a failed update")
}

func TestFabric_UpdateFabric_InvalidName(t *testing.T) {
	// --- Arrange ---
	fabric, err := NewFabric("TESTCODE", "Original Name", "m", "available")
	require.NoError(t, err)
	correctVersion := fabric.Version

	// --- Act ---
	// Attempt to update with an invalid name
	err = fabric.UpdateFabric("", "cm", "new_status", correctVersion)

	// --- Assert ---
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidFabricNameLength)
	assert.Equal(t, correctVersion, fabric.Version, "Version should not change on a failed update")
	assert.Len(t, fabric.events, 1, "No new event should be added on a failed update")
}

func TestNewFabric_ValidInput_ShouldSucced(t *testing.T) {
	// --- Arrange ---
	code := "ZOYA"
	name := faker.Word()
	measureUnit := "mb"
	offerStatus := "prototyp"

	// --- Act ---
	fabric, err := NewFabric(code, name, measureUnit, offerStatus)

	// --- Assert ---
	assert.NoError(t, err)
	assert.NotNil(t, fabric)
	assert.Equal(t, fabric.Code, code)
	assert.Equal(t, fabric.Name, name)
	assert.Equal(t, fabric.MeasureUnit, measureUnit)
	assert.Equal(t, fabric.Version, 1)
}

func TestNewFabric_InvalidCodeInput_ShouldFail(t *testing.T) {
	// --- Arrange ---
	name := faker.Word()
	measureUnit := "mb"
	offerStatus := "prototyp"
	invalidCodes := []string{
		"ZOY_A",                           // invalid character _
		"zoya",                            // invalid characters - small letters
		"1234567890123456789012345678901", // > 30 characters
	}

	for _, code := range invalidCodes {
		t.Run("InvalidCode_"+code, func(t *testing.T) {
			// --- Act ---
			fabric, err := NewFabric(code, name, measureUnit, offerStatus)

			// --- Assert ---
			assert.Error(t, err, "NewFabric should fail for invalid code")
			assert.Nil(t, fabric, "NewFabric should be nil for invalid code")
		})
	}
}

func TestNewFabric_InvalidNameInput_ShouldFail(t *testing.T) {
	// --- Arrange ---
	code := "ZOYA"
	measureUnit := "mb"
	offerStatus := "prototyp"
	invalidNames := []string{
		"", // name cannot be an empty string
		generateRandomString(251),
	}

	for _, name := range invalidNames {
		t.Run("InvalidName_"+name, func(t *testing.T) {
			// --- Act ---
			fabric, err := NewFabric(code, name, measureUnit, offerStatus)

			// --- Assert ---
			assert.Error(t, err, "NewFabric should fail for invalid name")
			assert.Nil(t, fabric, "NewFabric should be nil for invlaid name")
		})
	}
}

func TestNewFabric_ShouldGenerateFabricCreatedEvent(t *testing.T) {
	// --- Arrange ---
	code := "ZOYA"
	name := "Zoya"
	measureUnit := "mb"
	offerStatus := "prototyp"

	// --- Act ---
	fabric, err := NewFabric(code, name, measureUnit, offerStatus)
	assert.NoError(t, err)

	events := fabric.Events()

	// --- Assert ---
	assert.NotEmpty(t, events)
	assert.Len(t, events, 1)

	event, ok := events[0].(FabricCreated)
	assert.True(t, ok, "The first event should be a FabricCreated event")
	assert.Equal(
		t,
		FabricCreated{
			Code:        code,
			Name:        name,
			MeasureUnit: measureUnit,
			OfferStatus: offerStatus,
			Version:     1,
		},
		event,
		"Event data should match fabric inputs",
	)
}

func generateRandomString(length int) string {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	chars := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	result := make([]rune, length)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

func TestFabric_Update_FailsOnDeletedFabric(t *testing.T) {
	// --- Arrange ---
	fabric, err := NewFabric("TESTCODE", "Original Name", "m", "available")
	require.NoError(t, err)

	// Manually set the fabric to a deleted state for the test
	fabric.Status = StatusDeleted
	fabric.Version++ // Simulate a version increment from the delete operation

	// --- Act ---
	err = fabric.UpdateFabric("Attempted Update", "cm", "new", fabric.Version)

	// --- Assert ---
	assert.Error(t, err, "Should not be able to update a deleted fabric")
	assert.ErrorIs(t, err, ErrFabricDeleted)
}

func TestFabric_Delete_HappyPath(t *testing.T) {
	// --- Arrange ---
	fabric, err := NewFabric("TESTCODE", "Original Name", "m", "available")
	require.NoError(t, err)
	initialVersion := fabric.Version

	// --- Act ---
	err = fabric.Delete(initialVersion)

	// --- Assert ---
	assert.NoError(t, err)
	assert.Equal(t, StatusDeleted, fabric.Status)
	assert.Equal(t, initialVersion+1, fabric.Version)

	require.Len(t, fabric.events, 2, "Should have Created and Deleted events")
	deleteEvent, ok := fabric.events[1].(FabricDeleted)
	require.True(t, ok, "The second event must be a FabricDeleted event")
	assert.Equal(t, fabric.Code, deleteEvent.Code)
	assert.Equal(t, fabric.Version, deleteEvent.Version)
}

func TestFabric_Delete_ConcurrencyConflict(t *testing.T) {
	// --- Arrange ---
	fabric, err := NewFabric("TESTCODE", "Original Name", "m", "available")
	require.NoError(t, err)
	staleVersion := fabric.Version - 1

	// --- Act ---
	err = fabric.Delete(staleVersion)

	// --- Assert ---
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrConcurrencyConflict)
	assert.Equal(t, StatusActive, fabric.Status, "Status should not change on failed delete")
	assert.Len(t, fabric.events, 1, "No new event should be added on failed delete")
}

func TestFabric_Reactivate_HappyPath(t *testing.T) {
	// --- Arrange ---
	fabric, err := NewFabric("TESTCODE", "Original Name", "m", "available")
	require.NoError(t, err)
	// Manually set it to a deleted state for the test
	fabric.Status = StatusDeleted
	fabric.Version = 2 // Simulate a previous version increment from a delete op

	reactivatedName := "Reactivated Name"

	// --- Act ---
	err = fabric.Reactivate(reactivatedName, "m", "available", 2)

	// --- Assert ---
	assert.NoError(t, err)
	assert.Equal(t, StatusActive, fabric.Status)
	assert.Equal(t, reactivatedName, fabric.Name)
	assert.Equal(t, 3, fabric.Version)

	require.Len(t, fabric.events, 2, "Should have Created and Reactivated events")
	reactivateEvent, ok := fabric.events[1].(FabricReactivated)
	require.True(t, ok, "The second event must be a FabricReactivated event")
	assert.Equal(t, fabric.Code, reactivateEvent.Code)
	assert.Equal(t, fabric.Version, reactivateEvent.Version)
}
