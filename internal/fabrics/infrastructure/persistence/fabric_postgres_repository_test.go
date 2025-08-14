package persistence

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/salesworks/s-works/api/internal/fabrics/domain"
	"github.com/salesworks/s-works/api/internal/platform/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type postgresTestFixture struct {
	db   *database.PostgresDB
	repo *FabricPostgresRepository
	t    *testing.T
}

func setup(t *testing.T) *postgresTestFixture {
	t.Helper()

	db := setupTestPostgresDB(t)
	repo := NewFabricPostgresRepository(db)

	t.Cleanup(func() {
		_, err := db.Pool.Exec("DELETE FROM fabrics")
		if err != nil {
			t.Fatalf("Failed to clean up test data: %v", err)
		}
	})

	return &postgresTestFixture{
		db:   db,
		repo: repo,
		t:    t,
	}
}

func setupTestPostgresDB(t *testing.T) *database.PostgresDB {
	t.Helper()

	uri := os.Getenv("POSTGRES_URI")
	if uri == "" {
		t.Skip("Skipping integration test: POSTGRES_URI env variable is not ser")
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := database.NewPostgresDB(ctx, uri, 5, 5, 5*time.Minute, logger)
	require.NoError(t, err, "Failed to connect to postgres for error")

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

func TestFabricPostgresRepository_Save(t *testing.T) {
	// --- Arrange ---
	fixture := setup(t)
	fabricToSave, err := domain.NewFabric("PGTEST01", "Postgres Test Fabric", "m", "available")
	require.NoError(t, err)

	// --- Act ---
	_, err = fixture.repo.Save(context.Background(), fabricToSave)

	// --- Assert ---
	assert.NoError(t, err, "Save should not return an error")
	var version int
	err = fixture.db.Pool.QueryRow("SELECT version FROM fabrics WHERE code = $1", fabricToSave.Code).Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, 1, version, "the version should be set to 1 for newly created fabrics")
}

func TestFabricPostgresRepository_Save_ConflictOnActiveFabric(t *testing.T) {
	// --- Arrange ---
	fixture := setup(t)
	fabricToSave, err := domain.NewFabric("DUPLICATE", "Duplicate Test Fabric", "m", "available")
	require.NoError(t, err)

	// --- Act & Assert
	_, err = fixture.repo.Save(context.Background(), fabricToSave)
	assert.NoError(t, err, "First save should not return an error")

	_, err = fixture.repo.Save(context.Background(), fabricToSave)
	assert.ErrorIs(t, err, domain.ErrDuplicateFabricCode, "Error should be of type ErrDuplicateFabricCode for active records")
}

func TestFabricPostgresRepository_GetByCode(t *testing.T) {
	// --- Arrange ---
	fixture := setup(t)
	fabricToSave, err := domain.NewFabric("GETCODE", "GetByCode Fabric", "m", "available")
	require.NoError(t, err)

	// --- Act ---
	_, err = fixture.repo.Save(context.Background(), fabricToSave)
	require.NoError(t, err)
	retrivedFabric, err := fixture.repo.GetByCode(context.Background(), fabricToSave.Code)

	// --- Assert ---
	assert.NoError(t, err, "GetByCode should not return an error for an active fabric")
	assert.NotNil(t, retrivedFabric)
	assert.Equal(t, fabricToSave.Code, retrivedFabric.Code)
	assert.Equal(t, fabricToSave.Name, retrivedFabric.Name)

	_, err = fixture.repo.GetByCode(context.Background(), "NOEXISTENT")
	assert.ErrorIs(t, err, domain.ErrRecordNotFound, "GetByCode should return ErrRecordNotFound for nonexistent code")
}

func TestFabricPostgresRepository_Update_HappyPath(t *testing.T) {
	// --- Arrange ---
	fixture := setup(t)
	code := "UPDATETEST01"
	fabricToSave, err := domain.NewFabric(code, "Initial Name", "m", "available")
	require.NoError(t, err)

	_, err = fixture.repo.Save(context.Background(), fabricToSave)
	require.NoError(t, err, "Initial save should not fail")

	fabricToSave.Version++
	fabricToSave.Name = "Updated Fabric Name"

	// --- Act ---
	err = fixture.repo.Update(context.Background(), fabricToSave)

	// --- Assert ---
	assert.NoError(t, err, "Update should not return an error")

	var dbName string
	var dbVersion int
	query := "SELECT name, version FROM fabrics WHERE code = $1"
	err = fixture.db.Pool.QueryRow(query, code).Scan(&dbName, &dbVersion)
	require.NoError(t, err, "Should be able to query the updated fabric")
	assert.Equal(t, "Updated Fabric Name", dbName)
	assert.Equal(t, 2, dbVersion, "Version should be 2 after the update")
}

func TestFabricPostgresRepository_Update_ConcurrencyConflict(t *testing.T) {
	// --- Arrange ---
	fixture := setup(t)
	code := "UPDATETEST02"
	fabricToSave, err := domain.NewFabric(code, "Initial Name", "m", "available")
	require.NoError(t, err)

	_, err = fixture.repo.Save(context.Background(), fabricToSave)
	require.NoError(t, err)

	fabricToSave.Version = 0 // Stale version
	fabricToSave.Name = "This Should Not Be Saved"

	// --- Act ---
	err = fixture.repo.Update(context.Background(), fabricToSave)

	// --- Assert ---
	assert.Error(t, err, "Update should return an error due to version mismatch")
	assert.ErrorIs(t, err, domain.ErrRecordNotFound, "Error should indicate a concurrency issue (record not found at specified version)")
}

func TestFabricPostgresRepository_Delete(t *testing.T) {
	// --- Arrange ---
	fixture := setup(t)
	code := "DELETETEST"
	fabric, err := domain.NewFabric(code, "To Be Deleted", "m", "available")
	require.NoError(t, err)
	persistedFabric, err := fixture.repo.Save(context.Background(), fabric)
	require.NoError(t, err)

	// --- Act ---
	err = persistedFabric.Delete(1)
	require.NoError(t, err)

	err = fixture.repo.Delete(context.Background(), persistedFabric)
	require.NoError(t, err)

	// --- Assert ---
	_, err = fixture.repo.GetByCode(context.Background(), code)
	assert.ErrorIs(t, err, domain.ErrRecordNotFound, "GetByCode should not find a deleted fabric")

	var dbStatus string
	query := "SELECT status FROM fabrics WHERE code = $1"
	err = fixture.db.Pool.QueryRow(query, code).Scan(&dbStatus)
	require.NoError(t, err, "Should be able to query the deleted fabric directly")
	assert.Equal(t, domain.StatusDeleted, dbStatus)
}

func TestFabricPostgresRepository_Save_ReactivationIncrementsVersion(t *testing.T) {
	// --- Arrange ---
	fixture := setup(t)
	code := "REACTIVATE"

	// 1. Create a fabric (version 1)
	fabricToCreate, err := domain.NewFabric(code, "Original", "m", "available")
	require.NoError(t, err)
	persistedFabric, err := fixture.repo.Save(context.Background(), fabricToCreate)
	require.NoError(t, err)
	require.Equal(t, 1, persistedFabric.Version)

	// 2. Delete the fabric (results in version 2)
	err = persistedFabric.Delete(1)
	require.NoError(t, err)
	err = fixture.repo.Delete(context.Background(), persistedFabric)
	require.NoError(t, err)

	// 3. Prepare a "new" fabric object to simulate the reactivation request.
	reactivationRequest, err := domain.NewFabric(code, "Reactivated", "cm", "new")
	require.NoError(t, err)

	// --- Act ---
	finalFabric, err := fixture.repo.Save(context.Background(), reactivationRequest)

	// --- Assert ---
	require.NoError(t, err)
	require.NotNil(t, finalFabric)
	assert.Equal(t, 3, finalFabric.Version, "Reactivated fabric should have its version incremented from the deleted state, not reset to 1")
	assert.Equal(t, domain.StatusActive, finalFabric.Status)
	require.Len(t, finalFabric.Events(), 1)
	_, ok := finalFabric.Events()[0].(domain.FabricReactivated)
	assert.True(t, ok, "The event should be FabricReactivated")
}
